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

package consumer

import (
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
)

func newTestMessage(topic, msgID, traceID, segmentID string) *primitive.MessageExt {
	scx := core.SpanContext{
		Sample:                1,
		TraceID:               traceID,
		ParentSegmentID:       segmentID,
		ParentSpanID:          0,
		ParentService:         "producer-service",
		ParentServiceInstance: "producer-instance",
		ParentEndpoint:        "/producer/send",
		AddressUsedAtClient:   "mq.svc:9876",
	}
	msg := &primitive.MessageExt{MsgId: msgID, OffsetMsgId: "off-" + msgID}
	msg.Topic = topic
	msg.WithProperty(core.Header, scx.EncodeSW8())
	return msg
}

func waitReportedSpans(t *testing.T, want int) []reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		spans := core.GetReportedSpans()
		if len(spans) >= want {
			return spans
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d reported spans, got %d", want, len(spans))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func tagValue(s reporter.ReportedSpan, key string) string {
	for _, tag := range s.Tags() {
		if tag.Key == key {
			return tag.Value
		}
	}
	return ""
}

// TestBatchConsumeReportsSingleSpanWithAllRefs pins the Java-aligned batch
// semantics: ONE entry span carrying one segment reference per message, and
// it must be reported exactly once. The previous per-message CreateEntrySpan
// loop left the span reuse counter unbalanced (N reuses, one End), so the
// span of a batch with more than one message was never reported at all.
func TestBatchConsumeReportsSingleSpanWithAllRefs(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	msgs := []*primitive.MessageExt{
		newTestMessage("TopicTest", "m1", "11d1aaaaaaaaaaaaaaaaaaaaaaaaaaaa", "11c1aaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		newTestMessage("TopicTest", "m2", "22d2bbbbbbbbbbbbbbbbbbbbbbbbbbbb", "22c2bbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		newTestMessage("TopicTest", "m3", "33d3cccccccccccccccccccccccccccc", "33c3cccccccccccccccccccccccccccc"),
	}

	span, err := createConsumerEntrySpan(msgs, "nameserver:9876")
	if err != nil {
		t.Fatal(err)
	}
	if span == nil {
		t.Fatal("no span created for non-empty batch")
	}

	invocation := operator.NewInvocation(nil)
	invocation.SetContext(span)
	interceptor := &SwConsumerInterceptor{}
	if err := interceptor.AfterInvoke(invocation, consumer.ConsumeSuccess, nil); err != nil {
		t.Fatal(err)
	}

	spans := waitReportedSpans(t, 1)
	if len(spans) != 1 {
		t.Fatalf("batch must report exactly one span, got %d", len(spans))
	}
	reported := spans[0]
	if reported.OperationName() != "RocketMQ/TopicTest/Consumer" {
		t.Fatalf("unexpected operation name %s", reported.OperationName())
	}
	refs := reported.Refs()
	if len(refs) != 3 {
		t.Fatalf("every message of the batch must become a segment reference, got %d", len(refs))
	}
	if refs[0].GetTraceID() != "11d1aaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		refs[1].GetTraceID() != "22d2bbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
		refs[2].GetTraceID() != "33d3cccccccccccccccccccccccccccc" {
		t.Fatalf("refs do not carry the upstream traces in order: %v",
			[]string{refs[0].GetTraceID(), refs[1].GetTraceID(), refs[2].GetTraceID()})
	}
	if got := tagValue(reported, tagMQMsgID); got != "m1;m2;m3" {
		t.Fatalf("message ids must be aggregated, got %q", got)
	}
	if got := tagValue(reported, tagMQOffsetMsgID); got != "off-m1;off-m2;off-m3" {
		t.Fatalf("offset message ids must be aggregated, got %q", got)
	}
}

// TestEmptyBatchCreatesNoSpan keeps the empty-callback guard.
func TestEmptyBatchCreatesNoSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := createConsumerEntrySpan(nil, "nameserver:9876")
	if err != nil || span != nil {
		t.Fatalf("empty batch must be a no-op, span=%v err=%v", span, err)
	}
}
