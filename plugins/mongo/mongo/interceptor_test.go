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

package mongo

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

func installMonitor(t *testing.T) *event.CommandMonitor {
	t.Helper()
	opts := []*options.ClientOptions{{Hosts: []string{"127.0.0.1:27017"}}}
	interceptor := &NewClientInterceptor{}
	if err := interceptor.BeforeInvoke(operator.NewInvocation(nil, opts)); err != nil {
		t.Fatal(err)
	}
	if opts[0].Monitor == nil {
		t.Fatal("command monitor was not installed")
	}
	return opts[0].Monitor
}

func waitOneSpan(t *testing.T) reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if spans := core.GetReportedSpans(); len(spans) >= 1 {
			return spans[0]
		}
		if time.Now().After(deadline) {
			t.Fatal("span was never reported")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestCommandFinishedOnAnotherGoroutine replicates the SDAM/monitor topology:
// Started fires on goroutine A, Failed on goroutine B. The span completion
// must go through the async machinery (a plain End from B used to leave A's
// active stack pointing at a span another goroutine handed to the reporter).
func TestCommandFinishedOnAnotherGoroutine(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	monitor := installMonitor(t)

	started := make(chan struct{})
	go func() { // goroutine A: the operation goroutine
		defer close(started)
		monitor.Started(context.Background(), &event.CommandStartedEvent{
			CommandName:  "find",
			RequestID:    42,
			ConnectionID: "127.0.0.1:27017[-4]",
		})
		// the handed-over span must NOT stay active on this goroutine:
		// whatever the application starts next must not chain onto it
		if tracing.ActiveSpan() != nil {
			t.Error("the mongo span must be popped off the active stack after Started")
		}
	}()
	<-started

	failed := make(chan struct{})
	go func() { // goroutine B: a different SDK goroutine completes the command
		defer close(failed)
		monitor.Failed(context.Background(), &event.CommandFailedEvent{
			CommandFinishedEvent: event.CommandFinishedEvent{RequestID: 42},
			Failure:              "network error",
		})
	}()
	<-failed

	span := waitOneSpan(t)
	if span.OperationName() != "MongoDB/find" {
		t.Fatalf("unexpected operation name %s", span.OperationName())
	}
	if !span.IsError() {
		t.Fatal("a failed command must be an error span")
	}
	if span.EndTime() <= 0 {
		t.Fatal("span must carry the completion time")
	}
}

// TestCommandSucceededReportsOnce covers the happy path plus the
// unknown-request guard.
func TestCommandSucceededReportsOnce(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	monitor := installMonitor(t)

	monitor.Started(context.Background(), &event.CommandStartedEvent{
		CommandName: "insert",
		RequestID:   7,
	})
	// completion of a request the agent never saw must be a no-op
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{RequestID: 999},
	})
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{RequestID: 7},
	})
	// a duplicated completion must be a no-op as well (the map entry is gone)
	monitor.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{RequestID: 7},
	})

	span := waitOneSpan(t)
	if span.OperationName() != "MongoDB/insert" {
		t.Fatalf("unexpected operation name %s", span.OperationName())
	}
	if span.IsError() {
		t.Fatal("a succeeded command must not be an error span")
	}
	time.Sleep(100 * time.Millisecond)
	if got := len(core.GetReportedSpans()); got != 1 {
		t.Fatalf("the command span must be reported exactly once, got %d", got)
	}
}
