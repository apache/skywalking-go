// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package entry

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
	"github.com/apache/skywalking-go/plugins/gorm/mysql"
)

// okConnPool executes every statement successfully with one affected row,
// creating and ending an exit span like the database/sql plugin does. Inside
// the gorm exit span, that span is the gorm span itself (the exit span is
// reused), and its End makes the parent the active span again.
type okConnPool struct{ TestConnPool }

func (*okConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	span, err := tracing.CreateExitSpan("Mysql/Exec", peerAddress, func(k, v string) error {
		return nil
	})
	if err == nil {
		span.End()
	}
	return driver.RowsAffected(1), nil
}

type okDialector struct{ *TestDialector }

func (d okDialector) Initialize(db *gorm.DB) error {
	_ = d.TestDialector.Initialize(db)
	db.ConnPool = &okConnPool{}
	return nil
}

const (
	hookPanic      = "after create hook failed"
	hookUserCreate = "hook_users/create"
)

// panicHookUser has no auto-increment key, so creating it is a plain Exec.
type panicHookUser struct {
	Name string
}

func (*panicHookUser) AfterCreate(*gorm.DB) error {
	panic(hookPanic)
}

type hookUser struct {
	Name string
}

func (*hookUser) AfterCreate(*gorm.DB) error {
	return nil
}

func openTracedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(okDialector{NewTestDialector(&mysql.DatabaseInfo{PeerAddress: peerAddress})},
		&gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&OpenInterceptor{}).AfterInvoke(nil, db, nil); err != nil {
		t.Fatal(err)
	}
	return db
}

type executor interface {
	Execute(db *gorm.DB) *gorm.DB
}

// executeLikeAgent runs (*processor).Execute the way the instrumented code
// does: the after-hook is deferred, so it also runs while a callback panics,
// and its own panics are recovered and only logged.
func executeLikeAgent(t *testing.T, p executor, tx *gorm.DB) (ret *gorm.DB) {
	t.Helper()
	interceptor := &ExecuteInterceptor{}
	invocation := operator.NewInvocation(p, tx)
	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	defer func() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("the after-hook panicked: %v", r)
				}
			}()
			if err := interceptor.AfterInvoke(invocation, ret); err != nil {
				t.Errorf("the after-hook failed: %v", err)
			}
		}()
	}()
	return p.Execute(tx)
}

func waitSpans(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < want {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d reported spans, got %d", want, len(core.GetReportedSpans()))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func findSpan(name string) reporter.ReportedSpan {
	for _, s := range core.GetReportedSpans() {
		if s.OperationName() == name {
			return s
		}
	}
	return nil
}

// TestHookPanicEndsSpan covers a panic in a gorm hook: gorm runs the callback
// chain without defer, so the plugin's end callback is skipped. The span must
// still end, as an error, so that the request's segment is reported, and the
// application must still see the panic.
func TestHookPanicEndsSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	request, err := tracing.CreateEntrySpan("GET:/execute", func(headerKey string) (string, error) {
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	db := openTracedDB(t)
	// the same statement setup as db.Create(&panicHookUser{...})
	tx := db.Model(&panicHookUser{Name: "jinzhu"})

	recovered := func() (r interface{}) {
		defer func() { r = recover() }()
		executeLikeAgent(t, db.Callback().Create(), tx)
		return nil
	}()
	if recovered != hookPanic {
		t.Fatalf("the application must still see the hook panic, got %v", recovered)
	}
	request.End()

	waitSpans(t, 2)
	span := findSpan("panic_hook_users/create")
	if span == nil {
		t.Fatal("the gorm span was not reported")
	}
	if !span.IsError() {
		t.Fatal("an operation interrupted by a panic must be an error span")
	}
	var statement string
	for _, tag := range span.Tags() {
		if tag.Key == tracing.TagDBStatement {
			statement = tag.Value
		}
	}
	if statement != "INSERT INTO `panic_hook_users` (`name`) VALUES (?)" {
		t.Fatalf("unexpected statement %q", statement)
	}
}

// TestExecuteHookKeepsNormalSpan checks that the Execute hook does not end
// or report the span a second time when the end callback has run.
func TestExecuteHookKeepsNormalSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	db := openTracedDB(t)
	tx := db.Model(&hookUser{Name: "jinzhu"})
	if res := executeLikeAgent(t, db.Callback().Create(), tx); res.Error != nil {
		t.Fatal(res.Error)
	}

	waitSpans(t, 1)
	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected exactly one reported span, got %d", len(spans))
	}
	if spans[0].OperationName() != hookUserCreate || spans[0].IsError() {
		t.Fatalf("unexpected span %s, error %v", spans[0].OperationName(), spans[0].IsError())
	}
}

// TestExecuteHookEndsOnlyItsOwnSpan pins the ownership check: the hook ends
// only the span that the start callback created during its own Execute call
// on this goroutine. A statement shared across goroutines (unsupported by
// gorm) can hold the span of another goroutine.
func TestExecuteHookEndsOnlyItsOwnSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	db := newCallbackDB(&capturingLogger{})
	// a span started outside of the Execute call below
	beforeCallback(fakeDBInfo{}, "query")(db)

	interceptor := &ExecuteInterceptor{}
	invocation := operator.NewInvocation(nil, db)
	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if err := interceptor.AfterInvoke(invocation, nil); err != nil {
		t.Fatal(err)
	}
	if statementSpan(db) == nil {
		t.Fatal("a span that this Execute call did not start must stay on the statement")
	}

	afterCallback(fakeDBInfo{})(db)
	waitSpans(t, 1)
	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	if len(spans) != 1 || spans[0].IsError() {
		t.Fatalf("expected one successful span, got %d", len(spans))
	}
}

// withContextScope is a scope that returns a session, as scopes commonly do.
// gorm then runs the callbacks with a *gorm.DB whose statement must not be
// cloned again to store the span.
func withContextScope(tx *gorm.DB) *gorm.DB {
	return tx.WithContext(context.Background())
}

// TestScopeSessionSpanIsEnded covers an operation with a scope that returns a
// session: the end callback must find and end the span of the start callback.
func TestScopeSessionSpanIsEnded(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	db := openTracedDB(t)
	tx := db.Model(&hookUser{Name: "jinzhu"}).Scopes(withContextScope)
	if res := executeLikeAgent(t, db.Callback().Create(), tx); res.Error != nil {
		t.Fatal(res.Error)
	}

	waitSpans(t, 1)
	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	if len(spans) != 1 || spans[0].OperationName() != hookUserCreate || spans[0].IsError() {
		t.Fatalf("expected one successful hook_users/create span, got %d", len(spans))
	}
}

// TestScopeSessionHookPanicEndsSpan covers a hook panic in an operation with
// a scope that returns a session.
func TestScopeSessionHookPanicEndsSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	db := openTracedDB(t)
	tx := db.Model(&panicHookUser{Name: "jinzhu"}).Scopes(withContextScope)
	recovered := func() (r interface{}) {
		defer func() { r = recover() }()
		executeLikeAgent(t, db.Callback().Create(), tx)
		return nil
	}()
	if recovered != hookPanic {
		t.Fatalf("the application must still see the hook panic, got %v", recovered)
	}

	waitSpans(t, 1)
	span := findSpan("panic_hook_users/create")
	if span == nil || !span.IsError() {
		t.Fatal("the gorm span must be reported as an error")
	}
}
