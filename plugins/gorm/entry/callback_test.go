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
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/apache/skywalking-go/plugins/core"
)

type fakeDBInfo struct{}

func (fakeDBInfo) Type() string       { return "mysql" }
func (fakeDBInfo) ComponentID() int32 { return 5012 }
func (fakeDBInfo) Peer() string       { return "localhost:3306" }

type capturingLogger struct {
	mu    sync.Mutex
	warns []string
}

func (l *capturingLogger) LogMode(logger.LogLevel) logger.Interface { return l }
func (l *capturingLogger) Info(context.Context, string, ...interface{}) {
}
func (l *capturingLogger) Warn(_ context.Context, msg string, _ ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, msg)
}
func (l *capturingLogger) Error(context.Context, string, ...interface{}) {
}
func (l *capturingLogger) Trace(context.Context, time.Time, func() (string, int64), error) {
}

func (l *capturingLogger) warnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.warns)
}

func newCallbackDB(log logger.Interface) *gorm.DB {
	db := &gorm.DB{Config: &gorm.Config{Logger: log}}
	// Statement embeds *DB (promoted fields like Statement.Error resolve
	// through it), the back-reference is mandatory like in gorm.Open
	db.Statement = &gorm.Statement{DB: db, Table: "users"}
	return db
}

// waitSpanCount polls until the reported span count reaches want (the segment
// collection is asynchronous); raw sleeps flake under -race on slow runners.
func waitSpanCount(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < want {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d reported spans, got %d", want, len(core.GetReportedSpans()))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestAfterConsumesSpanAndClearsStatement covers the normal pair: the span is
// stored per-statement, reported by the after callback, and a following
// operation on the same statement must not see it as a leftover.
func TestAfterConsumesSpanAndClearsStatement(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	log := &capturingLogger{}
	db := newCallbackDB(log)
	before := beforeCallback(fakeDBInfo{}, "create")
	after := afterCallback(fakeDBInfo{})

	before(db)
	after(db)

	waitSpanCount(t, 1)
	spans := core.GetReportedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected one reported span, got %d", len(spans))
	}
	if spans[0].OperationName() != "users/create" {
		t.Fatalf("unexpected operation name %s", spans[0].OperationName())
	}

	// the same statement is reused for the next operation: no leftover warning
	before(db)
	if log.warnCount() != 0 {
		t.Fatalf("consumed span must not be reported as leftover: %v", log.warns)
	}
	after(db)
}

// TestLeftoverSpanIsReported makes the cross-goroutine *gorm.DB sharing
// misuse visible: two before callbacks on the same statement without an after
// in between mean the first span is overwritten and lost.
func TestLeftoverSpanIsReported(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	log := &capturingLogger{}
	db := newCallbackDB(log)
	before := beforeCallback(fakeDBInfo{}, "create")

	before(db)
	before(db) // overwrites the unfinished span of the first operation

	if log.warnCount() != 1 {
		t.Fatalf("expected exactly one leftover warning, got %d (%v)", log.warnCount(), log.warns)
	}
	if !strings.Contains(log.warns[0], "shared across goroutines") {
		t.Fatalf("warning must explain the misuse: %q", log.warns[0])
	}
	// drain the second span so the next test starts clean
	afterCallback(fakeDBInfo{})(db)
}

// TestSpanNotInheritedByClonedStatement pins the InstanceSet keying: gorm's
// Statement.clone copies the plain Settings into every Session/Transaction
// clone, which previously let a derived statement pick up - and end - the
// OUTER operation's span. The instance key contains the Statement pointer, so
// the clone must miss it.
func TestSpanNotInheritedByClonedStatement(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	log := &capturingLogger{}
	db := newCallbackDB(log)
	before := beforeCallback(fakeDBInfo{}, "create")
	after := afterCallback(fakeDBInfo{})

	before(db)

	// simulate gorm's Statement.clone: a NEW statement with the Settings
	// entries copied over (statement.go copies them one by one)
	cloned := newCallbackDB(log)
	db.Statement.Settings.Range(func(k, v interface{}) bool {
		cloned.Statement.Settings.Store(k, v)
		return true
	})

	after(cloned) // must NOT find - and end - the outer operation's span

	// negative check keeps a fixed window: nothing may arrive at all
	time.Sleep(100 * time.Millisecond)
	if got := len(core.GetReportedSpans()); got != 0 {
		t.Fatalf("the cloned statement must not end the outer span, got %d reported", got)
	}

	after(db) // the real owner ends it
	waitSpanCount(t, 1)
	if got := len(core.GetReportedSpans()); got != 1 {
		t.Fatalf("expected the outer span to be reported once, got %d", got)
	}
}
