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

package producer

import (
	"errors"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

func findCallbackSpan(t *testing.T, name string) reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, s := range core.GetReportedSpans() {
			if s.OperationName() == name {
				return s
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("callback span %q was never reported", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func callbackTagValue(s reporter.ReportedSpan, key string) (string, bool) {
	for _, tag := range s.Tags() {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

// runOnSDKGoroutine mirrors production: the send callback fires on a fresh
// SDK goroutine after the caller already ended the exit span.
func runOnSDKGoroutine(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	<-done
}

// TestAsyncCallbackFailedSendIsSafe pins the failed-send branch: sendResult is
// nil (the old code dereferenced it and killed the process - no recover exists
// on SDK goroutines) and the error must land on the CALLBACK local span, never
// on the already-ended exit span.
func TestAsyncCallbackFailedSendIsSafe(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := tracing.CreateExitSpan("RocketMQ/TopicTest/AsyncProducer", "nameserver:9876",
		func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	snapshot := tracing.CaptureContext()
	span.End() // AfterInvoke ends the exit span before the callback fires

	runOnSDKGoroutine(func() {
		traceAsyncSendCallback(snapshot, "TopicTest", "nameserver:9876", nil,
			errors.New("send to broker failed"),
			func(string) string {
				t.Error("broker lookup must not run for a failed send")
				return ""
			})
	})

	local := findCallbackSpan(t, "RocketMQ/TopicTest/Producer/Callback")
	if !local.IsError() {
		t.Fatal("send error must be recorded on the callback local span")
	}
	if _, ok := callbackTagValue(local, tracing.TagMQStatus); ok {
		t.Fatal("failed send must not carry a status tag")
	}
}

// TestAsyncCallbackSuccessTagsResult covers the happy path tag set.
func TestAsyncCallbackSuccessTagsResult(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := tracing.CreateExitSpan("RocketMQ/TopicTest/AsyncProducer", "nameserver:9876",
		func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	snapshot := tracing.CaptureContext()
	span.End()

	result := &primitive.SendResult{
		Status:      primitive.SendOK,
		MsgID:       "msg-1",
		OffsetMsgID: "off-1",
		MessageQueue: &primitive.MessageQueue{
			Topic:      "TopicTest",
			BrokerName: "broker-a",
			QueueId:    3,
		},
	}
	runOnSDKGoroutine(func() {
		traceAsyncSendCallback(snapshot, "TopicTest", "nameserver:9876", result, nil,
			func(brokerName string) string { return brokerName + ":10911" })
	})

	local := findCallbackSpan(t, "RocketMQ/TopicTest/Producer/Callback")
	if local.IsError() {
		t.Fatal("successful send must not be an error span")
	}
	if v, _ := callbackTagValue(local, tracing.TagMQBroker); v != "broker-a:10911" {
		t.Fatalf("broker tag mismatch: %q", v)
	}
	if v, _ := callbackTagValue(local, tracing.TagMQMsgID); v != "msg-1" {
		t.Fatalf("msg id tag mismatch: %q", v)
	}
	if v, _ := callbackTagValue(local, aSyncTagMQOffsetMsgID); v != "off-1" {
		t.Fatalf("offset msg id tag mismatch: %q", v)
	}
}

// TestAsyncCallbackAgentPanicIsIsolated proves a panic inside the agent logic
// (here: the broker lookup) never escapes to the user callback.
func TestAsyncCallbackAgentPanicIsIsolated(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := tracing.CreateExitSpan("RocketMQ/TopicTest/AsyncProducer", "nameserver:9876",
		func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	snapshot := tracing.CaptureContext()
	span.End()

	result := &primitive.SendResult{
		Status:       primitive.SendOK,
		MessageQueue: &primitive.MessageQueue{BrokerName: "broker-a"},
	}
	runOnSDKGoroutine(func() {
		// must return normally even though the agent logic panics inside
		traceAsyncSendCallback(snapshot, "TopicTest", "nameserver:9876", result, nil,
			func(string) string { panic("name server gone") })
	})
}
