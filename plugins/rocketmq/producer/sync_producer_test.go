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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type fakeNamesrvs struct{}

func (fakeNamesrvs) FindBrokerAddrByName(brokerName string) string { return brokerName + ":10911" }
func (fakeNamesrvs) AddrList() []string                            { return []string{"nameserver:9876"} }

type fakeRMQClient struct{}

func (fakeRMQClient) GetNameSrv() nativeNamesrvs { return fakeNamesrvs{} }

func newTestProducer() *nativedefaultProducer {
	return &nativedefaultProducer{client: fakeRMQClient{}}
}

// invokeLikeAgent runs an interceptor method the way the generated wrapper
// does: a panic is recovered (and logged in production). It returns the
// recovered value so that tests can assert that no panic happened.
func invokeLikeAgent(fn func() error) (recovered interface{}) {
	defer func() {
		recovered = recover()
	}()
	_ = fn()
	return nil
}

// TestSendSyncFailedEndsSpan (#2): on a failed send the SDK returns a non-nil
// SendResult with a nil MessageQueue (primitive.NewSendResult, filled only on
// success). The exit span must still end and record the error. Otherwise it
// stays active, and the whole segment is never reported.
func TestSendSyncFailedEndsSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	msgs := []*primitive.Message{primitive.NewMessage("TopicTest", []byte("m"))}
	inv := operator.NewInvocation(newTestProducer(), context.Background(), msgs)
	interceptor := &SendSyncInterceptor{}
	if r := invokeLikeAgent(func() error { return interceptor.BeforeInvoke(inv) }); r != nil {
		t.Fatalf("BeforeInvoke panicked: %v", r)
	}
	if r := invokeLikeAgent(func() error {
		return interceptor.AfterInvoke(inv, primitive.NewSendResult(), errors.New("CODE: 13, DESC: message illegal"))
	}); r != nil {
		t.Errorf("AfterInvoke panicked on a failed send: %v", r)
	}

	if span := tracing.ActiveSpan(); span != nil {
		t.Fatal("the exit span of the failed send was left active")
	}
	span := findCallbackSpan(t, "RocketMQ/TopicTest/Producer")
	if !span.IsError() {
		t.Fatal("the send error was not recorded")
	}
	if v, _ := callbackTagValue(span, tracing.TagMQStatus); v != "SendUnknownError" {
		t.Fatalf("mq.status: got %q", v)
	}
	if _, ok := callbackTagValue(span, tracing.TagMQQueue); ok {
		t.Fatal("a failed send has no queue")
	}
}

// TestSendSyncSuccessTagsResult keeps the tags of a successful send.
func TestSendSyncSuccessTagsResult(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	msgs := []*primitive.Message{primitive.NewMessage("TopicTest", []byte("m"))}
	inv := operator.NewInvocation(newTestProducer(), context.Background(), msgs)
	interceptor := &SendSyncInterceptor{}
	if err := interceptor.BeforeInvoke(inv); err != nil {
		t.Fatal(err)
	}
	result := &primitive.SendResult{
		Status:       primitive.SendOK,
		MsgID:        "msg-1",
		OffsetMsgID:  "off-1",
		MessageQueue: &primitive.MessageQueue{Topic: "TopicTest", BrokerName: "broker-a", QueueId: 3},
	}
	if err := interceptor.AfterInvoke(inv, result, nil); err != nil {
		t.Fatal(err)
	}

	span := findCallbackSpan(t, "RocketMQ/TopicTest/Producer")
	if span.IsError() {
		t.Fatal("a successful send must not be an error span")
	}
	if span.Peer() != "nameserver:9876" {
		t.Fatalf("peer: got %q", span.Peer())
	}
	want := map[string]string{
		tracing.TagMQStatus:  "SendOK",
		tracing.TagMQQueue:   "3",
		tracing.TagMQBroker:  "broker-a:10911",
		tracing.TagMQMsgID:   "msg-1",
		syncTagMQOffsetMsgID: "off-1",
	}
	for k, v := range want {
		if got, _ := callbackTagValue(span, k); got != v {
			t.Fatalf("tag %s: got %q, want %q", k, got, v)
		}
	}
	if msgs[0].GetProperty("sw8") == "" {
		t.Fatal("the sw8 header was not injected into the message")
	}
}

// TestEmptySendNoPanic (#30): SendSync, SendOneWay and SendAsync with no
// messages return ErrMessageEmpty from the SDK. The interceptors must not
// read msgList[0] first, and create no span.
func TestEmptySendNoPanic(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	var noMessages []*primitive.Message
	callback := func(ctx context.Context, result *primitive.SendResult, err error) {}
	cases := []struct {
		name        string
		interceptor operator.Interceptor
		inv         operator.Invocation
	}{
		{"SendSync", &SendSyncInterceptor{}, operator.NewInvocation(newTestProducer(), context.Background(), noMessages)},
		{"SendOneWay", &SendOneWayInterceptor{}, operator.NewInvocation(newTestProducer(), context.Background(), noMessages)},
		{"SendAsync", &SendASyncInterceptor{}, operator.NewInvocation(newTestProducer(), context.Background(), callback, noMessages)},
	}
	for _, c := range cases {
		if r := invokeLikeAgent(func() error { return c.interceptor.BeforeInvoke(c.inv) }); r != nil {
			t.Errorf("%s BeforeInvoke panicked on an empty send: %v", c.name, r)
		}
		if c.inv.GetContext() != nil || tracing.ActiveSpan() != nil {
			t.Errorf("%s created a span for an empty send", c.name)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if spans := core.GetReportedSpans(); len(spans) != 0 {
		t.Fatalf("expected no span, got %d", len(spans))
	}
}
