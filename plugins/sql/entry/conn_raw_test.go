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
	"errors"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type testInstanceInfo struct{}

func (testInstanceInfo) Peer() string       { return "mysql-server:3306" }
func (testInstanceInfo) ComponentID() int32 { return 5012 }
func (testInstanceInfo) DBType() string     { return "Mysql" }

// testConn stands in for an enhanced *sql.Conn carrying the instance info.
type testConn struct {
	field interface{}
}

func (c *testConn) GetSkyWalkingDynamicField() interface{}  { return c.field }
func (c *testConn) SetSkyWalkingDynamicField(v interface{}) { c.field = v }

// invokeLikeAgent runs an interceptor the way the generated code does: the
// after-hook is deferred and its panics are recovered and only logged.
func invokeLikeAgent(t *testing.T, interceptor operator.Interceptor, inv operator.Invocation, results ...interface{}) {
	t.Helper()
	if err := interceptor.BeforeInvoke(inv); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("the after-hook panicked: %v", r)
			}
		}()
		if err := interceptor.AfterInvoke(inv, results...); err != nil {
			t.Errorf("the after-hook failed: %v", err)
		}
	}()
}

func waitReportedSpan(t *testing.T) reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if spans := core.GetReportedSpans(); len(spans) >= 1 {
			return spans[0]
		}
		if time.Now().After(deadline) {
			t.Fatal("the span was never reported")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestConnRawEndsSpan covers (*sql.Conn).Raw, which returns only an error:
// reading a second result panicked before End, so the exit span stayed active
// and the whole segment was never reported.
func TestConnRawEndsSpan(t *testing.T) {
	for _, c := range []struct {
		name    string
		err     error
		isError bool
	}{
		{"success", nil, false},
		{"error", errors.New("raw failed"), true},
	} {
		t.Run(c.name, func(t *testing.T) {
			core.ResetTracingContext()
			defer core.ResetTracingContext()

			conn := &testConn{field: testInstanceInfo{}}
			raw := func(driverConn any) error { return nil }
			invokeLikeAgent(t, &ConnRawInterceptor{}, operator.NewInvocation(conn, raw), c.err)

			if tracing.ActiveSpan() != nil {
				t.Error("the Conn/Raw span must not stay active after the call")
			}
			span := waitReportedSpan(t)
			if span.OperationName() != "Mysql/Conn/Raw" {
				t.Fatalf("unexpected operation name %s", span.OperationName())
			}
			if span.IsError() != c.isError {
				t.Fatalf("isError: want %v, got %v", c.isError, span.IsError())
			}
		})
	}
}
