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
	"context"
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	testServiceURL = "pulsar://pulsar-server:6650"
	testTopic      = "t1"
	testFullTopic  = "persistent://public/default/t1"
)

// countingLookup stands in for the SDK lookup service. In the SDK every
// Lookup call is an uncached CommandLookupTopic RPC to a broker, so the
// interceptors must never call it.
type countingLookup struct {
	calls int32
}

func (l *countingLookup) Lookup(topic string) (*LookupResult, error) {
	atomic.AddInt32(&l.calls, 1)
	addr, err := url.Parse("pulsar://broker-from-lookup:6650")
	if err != nil {
		return nil, err
	}
	return &LookupResult{LogicalAddr: addr, PhysicalAddr: addr}, nil
}

func (l *countingLookup) assertNotCalled(t *testing.T) {
	t.Helper()
	if n := atomic.LoadInt32(&l.calls); n != 0 {
		t.Fatalf("the interceptor ran %d broker lookup RPC(s), want none", n)
	}
}

// At build time the agent adds these methods to the SDK client
// (NewStructEnhance("client")). They give the test mirror the same shape.
var testClientDynamicFields sync.Map

func (c *nativeclient) GetSkyWalkingDynamicField() interface{} {
	v, _ := testClientDynamicFields.Load(c)
	return v
}

func (c *nativeclient) SetSkyWalkingDynamicField(v interface{}) {
	testClientDynamicFields.Store(c, v)
}

// newTestClient returns a client mirror that carries its service URL, as
// NewClient leaves it in an instrumented application.
func newTestClient(t *testing.T, lookup nativeLookupService) *nativeclient {
	t.Helper()
	c := &nativeclient{lookupService: lookup}
	c.SetSkyWalkingDynamicField(testServiceURL)
	return c
}

func newTestProducer(t *testing.T, lookup nativeLookupService) *nativepartitionProducer {
	return &nativepartitionProducer{
		client:  newTestClient(t, lookup),
		topic:   testFullTopic,
		options: &nativeProducerOptions{Topic: testTopic},
	}
}

func newTestConsumer(t *testing.T, lookup nativeLookupService) *nativeconsumer {
	return &nativeconsumer{
		client:  newTestClient(t, lookup),
		topic:   testFullTopic,
		options: &nativeConsumerOptions{Topic: testTopic},
	}
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

// assertNoSpanReported waits briefly, because spans are reported by a
// collector goroutine after the segment ends.
func assertNoSpanReported(t *testing.T) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	if spans := core.GetReportedSpans(); len(spans) != 0 {
		t.Fatalf("expected no span, got %d, first: %q", len(spans), spans[0].OperationName())
	}
}

// fakeMessage implements pulsar.Message through the embedded interface and
// only overrides what the consumer interceptor reads.
type fakeMessage struct {
	pulsar.Message
	properties map[string]string
}

func (m *fakeMessage) Properties() map[string]string {
	return m.properties
}

// TestNewClientRecordsServiceURL checks that NewClient saves the service URL
// on the client, where the producer and consumer interceptors read the peer.
func TestNewClientRecordsServiceURL(t *testing.T) {
	c := &nativeclient{}
	inv := operator.NewInvocation(nil, pulsar.ClientOptions{URL: testServiceURL})
	if err := (&NewClientInterceptor{}).AfterInvoke(inv, c, nil); err != nil {
		t.Fatal(err)
	}
	if got := serviceURL(c); got != testServiceURL {
		t.Fatalf("service URL of the client: got %q, want %q", got, testServiceURL)
	}

	// a failed NewClient returns a nil Client: nothing to save, no panic
	if r := invokeLikeAgent(func() error {
		return (&NewClientInterceptor{}).AfterInvoke(inv, nil, errors.New("invalid URL"))
	}); r != nil {
		t.Fatalf("NewClientInterceptor panicked on a failed NewClient: %v", r)
	}
	// a client without a saved URL, or no client, gives an empty peer
	if got := serviceURL(&nativeclient{}); got != "" {
		t.Fatalf("service URL of a client without one: got %q", got)
	}
	if got := serviceURL(nil); got != "" {
		t.Fatalf("service URL of a nil client: got %q", got)
	}
}

// TestSendNoLookupRPC (#12): Send must not run a broker lookup RPC, and the
// span peer is the client's service URL.
func TestSendNoLookupRPC(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	lookup := &countingLookup{}
	msg := &pulsar.ProducerMessage{Payload: []byte("m")}
	inv := operator.NewInvocation(newTestProducer(t, lookup), context.Background(), msg)
	interceptor := &SendInterceptor{}
	if r := invokeLikeAgent(func() error { return interceptor.BeforeInvoke(inv) }); r != nil {
		t.Fatalf("BeforeInvoke panicked: %v", r)
	}
	if r := invokeLikeAgent(func() error { return interceptor.AfterInvoke(inv, nil, nil) }); r != nil {
		t.Fatalf("AfterInvoke panicked: %v", r)
	}

	lookup.assertNotCalled(t)
	span := findReportedSpan(t, "Pulsar/t1/Producer")
	if span.Peer() != testServiceURL {
		t.Fatalf("peer: got %q, want %q", span.Peer(), testServiceURL)
	}
	if v, _ := reportedTagValue(span, tracing.TagMQBroker); v != testServiceURL {
		t.Fatalf("mq.broker: got %q, want %q", v, testServiceURL)
	}
	if msg.Properties["sw8"] == "" {
		t.Fatal("the sw8 header was not injected into the message")
	}
}

// TestSendAsyncNoLookupRPC (#12): the same for SendAsync and its callback span.
func TestSendAsyncNoLookupRPC(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	lookup := &countingLookup{}
	userCalled := make(chan struct{}, 1)
	userCallback := func(id MessageID, message *ProducerMessage, err error) { userCalled <- struct{}{} }
	inv := operator.NewInvocation(newTestProducer(t, lookup), context.Background(), &ProducerMessage{}, userCallback)
	interceptor := &SendAsyncInterceptor{}
	if r := invokeLikeAgent(func() error { return interceptor.BeforeInvoke(inv) }); r != nil {
		t.Fatalf("BeforeInvoke panicked: %v", r)
	}
	wrapped, ok := inv.Args()[2].(func(MessageID, *ProducerMessage, error))
	if !ok || wrapped == nil {
		t.Fatal("the callback was not wrapped")
	}
	if r := invokeLikeAgent(func() error { return interceptor.AfterInvoke(inv) }); r != nil {
		t.Fatalf("AfterInvoke panicked: %v", r)
	}
	runOnSDKConnectionGoroutine(t, func() { wrapped(&fakeMessageID{id: "1:2:3"}, &ProducerMessage{}, nil) })

	select {
	case <-userCalled:
	default:
		t.Fatal("the user callback was not called")
	}
	lookup.assertNotCalled(t)
	exit := findReportedSpan(t, "Pulsar/t1/AsyncProducer")
	if exit.Peer() != testServiceURL {
		t.Fatalf("exit span peer: got %q, want %q", exit.Peer(), testServiceURL)
	}
	callback := findReportedSpan(t, "Pulsar/t1/Producer/Callback")
	if callback.Peer() != testServiceURL {
		t.Fatalf("callback span peer: got %q, want %q", callback.Peer(), testServiceURL)
	}
}

// runOnSDKConnectionGoroutine calls fn on a new goroutine, as the SDK does for
// send receipts. The SDK has no recover there, so a panic would kill the
// application. The test recovers it to report a failure instead.
func runOnSDKConnectionGoroutine(t *testing.T, fn func()) {
	t.Helper()
	panicked := make(chan interface{}, 1)
	go func() {
		defer func() { panicked <- recover() }()
		fn()
	}()
	if r := <-panicked; r != nil {
		t.Fatalf("the callback panicked on the SDK goroutine, which crashes the application: %v", r)
	}
}

// TestSendAsyncNilCallback (#1): producer.SendAsync(ctx, msg, nil) is valid
// SDK usage (the SDK skips a nil callback). The wrapped callback must not call
// the nil user callback, on success or on failure.
func TestSendAsyncNilCallback(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	inv := operator.NewInvocation(newTestProducer(t, &countingLookup{}), context.Background(), &ProducerMessage{},
		(func(MessageID, *ProducerMessage, error))(nil))
	interceptor := &SendAsyncInterceptor{}
	if r := invokeLikeAgent(func() error { return interceptor.BeforeInvoke(inv) }); r != nil {
		t.Fatalf("BeforeInvoke panicked: %v", r)
	}
	if r := invokeLikeAgent(func() error { return interceptor.AfterInvoke(inv) }); r != nil {
		t.Fatalf("AfterInvoke panicked: %v", r)
	}
	wrapped, ok := inv.Args()[2].(func(MessageID, *ProducerMessage, error))
	if !ok || wrapped == nil {
		t.Fatal("the callback was not wrapped")
	}

	// receipt of a successful send, then an inline failure
	runOnSDKConnectionGoroutine(t, func() { wrapped(&fakeMessageID{id: "1:2:3"}, &ProducerMessage{}, nil) })
	runOnSDKConnectionGoroutine(t, func() { wrapped(nil, &ProducerMessage{}, errors.New("producer closed")) })

	// the async result is still traced
	findReportedSpan(t, "Pulsar/t1/Producer/Callback")
}

// TestReceiveFailedNoSpan (#9): Receive returns (nil, err) when it fails,
// for example when the consumer is closed or the caller's ctx is done. The
// interceptor must not panic on the nil message, and creates no span: nothing
// was received, and an entry span would take over the caller's entry span.
func TestReceiveFailedNoSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	request, err := tracing.CreateEntrySpan("GET:/consume", func(string) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}
	inv := operator.NewInvocation(newTestConsumer(t, &countingLookup{}), context.Background())
	interceptor := &ReceiveInterceptor{}
	for _, receiveErr := range []error{errors.New("consumer closed"), context.DeadlineExceeded, context.Canceled} {
		if r := invokeLikeAgent(func() error { return interceptor.AfterInvoke(inv, nil, receiveErr) }); r != nil {
			t.Fatalf("AfterInvoke panicked on Receive error %v: %v", receiveErr, r)
		}
	}
	request.End()

	span := findReportedSpan(t, "GET:/consume")
	if span.IsError() || len(core.GetReportedSpans()) != 1 {
		t.Fatalf("the caller's entry span must be the only span, unchanged, got %d spans", len(core.GetReportedSpans()))
	}
}

// TestReceiveNoLookupRPC (#12): a received message creates the Entry span
// without a broker lookup RPC, and continues the producer's trace.
func TestReceiveNoLookupRPC(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	// the sw8 header of a producer exit span
	producerHeaders := map[string]string{}
	producerSpan, err := tracing.CreateExitSpan("Pulsar/t1/Producer", testServiceURL, func(k, v string) error {
		producerHeaders[k] = v
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	producerSpan.End()
	core.ResetTracingContext()

	lookup := &countingLookup{}
	inv := operator.NewInvocation(newTestConsumer(t, lookup), context.Background())
	interceptor := &ReceiveInterceptor{}
	msg := &fakeMessage{properties: producerHeaders}
	if r := invokeLikeAgent(func() error { return interceptor.AfterInvoke(inv, msg, nil) }); r != nil {
		t.Fatalf("AfterInvoke panicked: %v", r)
	}

	lookup.assertNotCalled(t)
	span := findReportedSpan(t, "Pulsar/t1/Consumer")
	if span.IsError() {
		t.Fatal("a received message must not be an error span")
	}
	if span.Peer() != testServiceURL {
		t.Fatalf("peer: got %q, want %q", span.Peer(), testServiceURL)
	}
	if len(span.Refs()) != 1 {
		t.Fatalf("the consumer span must continue the producer trace, refs: %d", len(span.Refs()))
	}
}
