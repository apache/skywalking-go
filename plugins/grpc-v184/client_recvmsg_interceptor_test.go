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

package grpcv184

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// The agent adds the dynamic field to the real clientStream at compile time;
// the tests give the mirror type the same EnhancedInstance contract.
var testClientStreamFields sync.Map

func (cs *nativeclientStream) GetSkyWalkingDynamicField() interface{} {
	v, _ := testClientStreamFields.Load(cs)
	return v
}

func (cs *nativeclientStream) SetSkyWalkingDynamicField(v interface{}) {
	testClientStreamFields.Store(cs, v)
}

func newTestClientStream(method string, desc *nativeStreamDesc) *nativeclientStream {
	return &nativeclientStream{callHdr: &nativeCallHdr{Method: method}, desc: desc}
}

func invokeInterceptor(t *testing.T, interceptor operator.Interceptor, invocation operator.Invocation, result ...interface{}) {
	t.Helper()
	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if err := interceptor.AfterInvoke(invocation, result...); err != nil {
		t.Fatal(err)
	}
}

// TestClientUnaryKeepsSpansOfOlderGrpc replays the clientStream calls of a
// unary invoke in grpc v1.84.0 (SendMsg, CloseSend, RecvMsg, trailer RecvMsg)
// and expects the spans grpc < v1.84.0 produced (SendMsg, RecvMsg).
func TestClientUnaryKeepsSpansOfOlderGrpc(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	exit, err := tracing.CreateExitSpan("api.Echo.UnaryEcho", "localhost:9999", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	cs := newTestClientStream(testUnaryMethod, &nativeStreamDesc{})
	invokeInterceptor(t, &ClientSendMsgInterceptor{}, operator.NewInvocation(cs, struct{}{}), error(nil))

	closeSend := operator.NewInvocation(cs)
	invokeInterceptor(t, &ClientCloseSendInterceptor{}, closeSend, error(nil))
	if closeSend.GetContext() != nil {
		t.Fatal("unary CloseSend must not create a span")
	}

	recv := &ClientRecvMsgInterceptor{}
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), error(nil))
	// finish is called inside the trailer read, it must stay a no-op for unary RPCs
	trailer := operator.NewInvocation(cs, struct{}{})
	if err := recv.BeforeInvoke(trailer); err != nil {
		t.Fatal(err)
	}
	invokeInterceptor(t, &ClientFinishInterceptor{}, operator.NewInvocation(cs, error(nil)))
	if err := recv.AfterInvoke(trailer, io.EOF); err != nil {
		t.Fatal(err)
	}
	exit.End()

	spans := waitReportedSpans(t, 3)
	findReportedSpan(t, spans, "api.Echo.UnaryEcho/Client/Request/SendMsg")
	recvSpan := findReportedSpan(t, spans, "api.Echo.UnaryEcho/Client/Response/RecvMsg")
	if recvSpan.IsError() {
		t.Fatal("the trailer io.EOF must not mark the RecvMsg span as error")
	}
	findReportedSpan(t, spans, "api.Echo.UnaryEcho")
}

func TestClientUnaryTrailerErrorMarksRecvSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	exit, err := tracing.CreateExitSpan("api.Echo.UnaryEcho", "localhost:9999", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	cs := newTestClientStream(testUnaryMethod, &nativeStreamDesc{})
	recv := &ClientRecvMsgInterceptor{}
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), error(nil))
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), errors.New("rpc error: code = Internal"))
	exit.End()

	spans := waitReportedSpans(t, 2)
	if !findReportedSpan(t, spans, "api.Echo.UnaryEcho/Client/Response/RecvMsg").IsError() {
		t.Fatal("a trailer error must mark the RecvMsg span as error")
	}
}

// TestClientUnaryCardinalityViolationMarksRecvSpan covers a second response
// message where grpc v1.84.0 expects the trailer: the trailer read returns nil
// and invoke reports the cardinality error after the hook.
func TestClientUnaryCardinalityViolationMarksRecvSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	exit, err := tracing.CreateExitSpan("api.Echo.UnaryEcho", "localhost:9999", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	cs := newTestClientStream(testUnaryMethod, &nativeStreamDesc{})
	recv := &ClientRecvMsgInterceptor{}
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), error(nil))
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), error(nil))
	exit.End()

	spans := waitReportedSpans(t, 2)
	if !findReportedSpan(t, spans, "api.Echo.UnaryEcho/Client/Response/RecvMsg").IsError() {
		t.Fatal("a second response message must mark the RecvMsg span as error")
	}
}

// TestClientStreamingCardinalityViolationFinishesStream covers a second
// response message in a client-streaming call: grpc v1.84.0 does not finish the
// stream, so the trailer read must finish the stream span with the error.
func TestClientStreamingCardinalityViolationFinishesStream(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	const method = "/api.Echo/ClientStreamingEcho"
	cs := newTestClientStream(method, &nativeStreamDesc{ClientStreams: true})
	exit, err := tracing.CreateExitSpan("api.Echo.ClientStreamingEcho", "localhost:9999", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), &nativeStreamDesc{ClientStreams: true}, method)
	newStream.SetContext(exit)
	if err := (&ClientStreamingInterceptor{}).AfterInvoke(newStream, &nativeclientStreamWrapper{ClientStream: cs}, error(nil)); err != nil {
		t.Fatal(err)
	}

	recv := &ClientRecvMsgInterceptor{}
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), error(nil))
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), error(nil))

	spans := waitReportedSpans(t, 2)
	if !findReportedSpan(t, spans, "api.Echo.ClientStreamingEcho").IsError() {
		t.Fatal("the stream span must be finished with the cardinality error")
	}
	if !findReportedSpan(t, spans, "api.Echo.ClientStreamingEcho/Client/Response/RecvMsg").IsError() {
		t.Fatal("the RecvMsg span must be marked with the cardinality error")
	}
	// a later finish from grpc must stay a no-op
	invokeInterceptor(t, &ClientFinishInterceptor{}, operator.NewInvocation(cs, error(nil)))
}

// TestClientStreamingKeepsSpansOfOlderGrpc replays a client-streaming
// CloseAndRecv in grpc v1.84.0: NewStream returns the clientStreamWrapper, and
// the wrapper reads the response and then the trailer, which finishes the
// stream. grpc < v1.84.0 reported a single CloseRecv span under the exit span.
func TestClientStreamingKeepsSpansOfOlderGrpc(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	const method = "/api.Echo/ClientStreamingEcho"
	cs := newTestClientStream(method, &nativeStreamDesc{ClientStreams: true})
	// the exit span ClientStreamingInterceptor.BeforeInvoke creates (the mirrored
	// ClientConn has no target to use as peer)
	exit, err := tracing.CreateExitSpan("api.Echo.ClientStreamingEcho", "localhost:9999", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), &nativeStreamDesc{ClientStreams: true}, method)
	newStream.SetContext(exit)
	if err := (&ClientStreamingInterceptor{}).AfterInvoke(newStream, &nativeclientStreamWrapper{ClientStream: cs}, error(nil)); err != nil {
		t.Fatal(err)
	}
	if clientStreamContextData(cs) == nil {
		t.Fatal("the context data must be attached to the clientStream behind the wrapper")
	}

	recv := &ClientRecvMsgInterceptor{}
	invokeInterceptor(t, recv, operator.NewInvocation(cs, struct{}{}), error(nil))
	trailer := operator.NewInvocation(cs, struct{}{})
	if err := recv.BeforeInvoke(trailer); err != nil {
		t.Fatal(err)
	}
	invokeInterceptor(t, &ClientFinishInterceptor{}, operator.NewInvocation(cs, error(nil)))
	if err := recv.AfterInvoke(trailer, io.EOF); err != nil {
		t.Fatal(err)
	}

	spans := waitReportedSpans(t, 2)
	exitSpan := findReportedSpan(t, spans, "api.Echo.ClientStreamingEcho")
	closeRecv := findReportedSpan(t, spans, "api.Echo.ClientStreamingEcho/Client/Response/CloseRecv")
	if closeRecv.Context().GetParentSpanID() != exitSpan.Context().GetSpanID() {
		t.Fatalf("CloseRecv parent span = %d, want the exit span %d",
			closeRecv.Context().GetParentSpanID(), exitSpan.Context().GetSpanID())
	}
	if closeRecv.IsError() {
		t.Fatal("CloseRecv must not be an error span")
	}
}

func TestUnwrapClientStream(t *testing.T) {
	cs := newTestClientStream(testUnaryMethod, &nativeStreamDesc{})
	if got := unwrapClientStream(&nativeclientStreamWrapper{ClientStream: cs}); got != cs {
		t.Fatalf("unwrap wrapper = %v, want the clientStream", got)
	}
	if got := unwrapClientStream(cs); got != cs {
		t.Fatalf("unwrap clientStream = %v, want itself", got)
	}
	if got := unwrapClientStream(nil); got != nil {
		t.Fatalf("unwrap nil = %v", got)
	}
}
