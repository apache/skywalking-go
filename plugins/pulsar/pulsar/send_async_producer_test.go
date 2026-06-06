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

package pulsar

import (
	"errors"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type fakeMessageID struct{ id string }

func (f *fakeMessageID) String() string { return f.id }

func findReportedSpan(t *testing.T, name string) reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, s := range core.GetReportedSpans() {
			if s.OperationName() == name {
				return s
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("span %q was never reported", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func reportedTagValue(s reporter.ReportedSpan, key string) (string, bool) {
	for _, tag := range s.Tags() {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

// TestAsyncCallbackFailedSendIsSafe pins the failed-send branch: the message
// id is nil (the old code called id.String() and killed the process - no
// recover exists on SDK goroutines) and the error must land on the CALLBACK
// local span, never on the already-ended exit span.
func TestAsyncCallbackFailedSendIsSafe(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := tracing.CreateExitSpan("Pulsar/persistent://public/default/t1/AsyncProducer", "broker:6650",
		func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	snapshot := tracing.CaptureContext()
	span.End() // AfterInvoke ends the exit span before the callback fires

	done := make(chan struct{})
	go func() { // the SDK callback goroutine
		defer close(done)
		traceAsyncSendCallback(snapshot, "t1", "persistent://public/default/t1",
			"broker:6650", "broker:6650", nil, errors.New("send failed"))
	}()
	<-done

	local := findReportedSpan(t, "Pulsar/t1/Producer/Callback")
	if !local.IsError() {
		t.Fatal("send error must be recorded on the callback local span")
	}
	if _, ok := reportedTagValue(local, tracing.TagMQMsgID); ok {
		t.Fatal("failed send must not carry a message id tag")
	}
	if v, _ := reportedTagValue(local, tracing.TagMQTopic); v != "persistent://public/default/t1" {
		t.Fatalf("topic tag mismatch: %q", v)
	}
}

// TestAsyncCallbackSuccessTagsResult covers the happy path tag set.
func TestAsyncCallbackSuccessTagsResult(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := tracing.CreateExitSpan("Pulsar/t1/AsyncProducer", "broker:6650",
		func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	snapshot := tracing.CaptureContext()
	span.End()

	done := make(chan struct{})
	go func() {
		defer close(done)
		traceAsyncSendCallback(snapshot, "t1", "t1", "broker:6650", "broker:6650",
			&fakeMessageID{id: "ledger:1:entry:2"}, nil)
	}()
	<-done

	local := findReportedSpan(t, "Pulsar/t1/Producer/Callback")
	if local.IsError() {
		t.Fatal("successful send must not be an error span")
	}
	if v, _ := reportedTagValue(local, tracing.TagMQMsgID); v != "ledger:1:entry:2" {
		t.Fatalf("msg id tag mismatch: %q", v)
	}
	if v, _ := reportedTagValue(local, tracing.TagMQBroker); v != "broker:6650" {
		t.Fatalf("broker tag mismatch: %q", v)
	}
}
