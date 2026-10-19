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
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// The tests below replay the client stream calls that grpc v1.84.0 makes, and
// run the interceptors that Points() registers for them, the way the
// instrumented grpc code does.

const (
	testStreamMethod    = "/api.Echo/ServerStreamingEcho"
	testStreamOperation = "api.Echo.ServerStreamingEcho"
	testEntryOperation  = "GET:/consumer"
)

// testWrappedStream is the stream a user stream interceptor returns.
type testWrappedStream struct {
	inner interface{}
}

// testClientStreamWrapper stands in for the clientStreamWrapper that grpc
// v1.84.0 returns from ClientConn.NewStream around the clientStream.
type testClientStreamWrapper struct {
	clientStream interface{}
}

// testInterceptors maps the interceptor names used in Points() to interceptors.
var testInterceptors = map[string]operator.Interceptor{
	"ClientStreamingInterceptor": &ClientStreamingInterceptor{},
	"ClientSendMsgInterceptor":   &ClientSendMsgInterceptor{},
	"ClientRecvMsgInterceptor":   &ClientRecvMsgInterceptor{},
	"ClientCloseSendInterceptor": &ClientCloseSendInterceptor{},
}

// hooked runs fn, which replays a grpc function, between the interceptor calls
// of that function. Only a function that Points() registers is intercepted,
// and the interceptor panics are recovered, as the generated adapter does, but
// fail the test.
func hooked(t *testing.T, receiver, name string, invocation operator.Invocation, fn func() []interface{}) []interface{} {
	t.Helper()
	var interceptor operator.Interceptor
	for _, point := range NewInstrument().Points() {
		if point.At.Type == instrument.EnhanceTypeMethod && point.At.Receiver == receiver && point.At.Name == name {
			if interceptor = testInterceptors[strings.TrimSpace(point.Interceptor)]; interceptor == nil {
				t.Fatalf("no test interceptor for %q", point.Interceptor)
			}
		}
	}
	if interceptor == nil {
		return fn()
	}
	runInterceptor(t, name, func() error { return interceptor.BeforeInvoke(invocation) })
	results := fn()
	runInterceptor(t, name, func() error { return interceptor.AfterInvoke(invocation, results...) })
	return results
}

func runInterceptor(t *testing.T, name string, call func() error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("the %s interceptor panicked: %v", name, r)
		}
	}()
	if err := call(); err != nil {
		t.Logf("the %s interceptor returned: %v", name, err)
	}
}

// testStream replays a client stream of grpc v1.84.0.
type testStream struct {
	t    *testing.T
	desc *nativeStreamDesc
	cs   *nativeclientStream
	// userOpts are the call options that the application passes
	userOpts []grpc.CallOption
	// opts are the call options of the clientStream, as newClientStreamWithParams
	// received them
	opts []grpc.CallOption
	// finished is whether grpc finished the clientStream, which it does once
	finished bool
	// watcher is the tracing context of the goroutine that newClientStreamWithParams
	// starts to finish the stream when the stream context or the ClientConn is done
	watcher interface{}
}

// newTestStream replays ClientConn.NewStream: the user stream interceptor, if
// any, calls defaultStreamInterceptor, which calls newClientStream, which
// creates the clientStream in newClientStreamWithParams, and wraps it in a
// clientStreamWrapper. A non-nil createErr fails the stream creation, and wrap
// stands for what the user stream interceptor does with the stream.
func newTestStream(t *testing.T, desc *nativeStreamDesc, createErr error,
	wrap func(s *testStream, stream interface{}) interface{}) (s *testStream, stream interface{}, err error) {
	s = &testStream{t: t, desc: desc}
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), desc, testStreamMethod, []grpc.CallOption(nil))
	results := hooked(t, "*ClientConn", "NewStream", newStream, func() []interface{} {
		created := s.create(newStream.Args()[0].(context.Context), createErr, nil)
		if created[1] != nil {
			return created
		}
		var stream interface{} = &testClientStreamWrapper{clientStream: created[0]}
		if wrap != nil {
			stream = wrap(s, stream)
		}
		return []interface{}{stream, nil}
	})
	err, _ = results[1].(error)
	return s, results[0], err
}

// create replays newClientStreamWithParams. A non-nil createErr fails the
// creation before an attempt exists, in which case grpc calls no OnFinish
// call option of the clientStream (newClientStream cleans up with its own
// options). started runs after the clientStream and its watcher goroutine
// exist, before newClientStreamWithParams returns.
func (s *testStream) create(ctx context.Context, createErr error, started func()) []interface{} {
	create := operator.NewInvocation(nil, ctx, s.desc, &nativeClientConn{}, testStreamMethod, nil, func() {}, false, s.userOpts)
	return hooked(s.t, "", "newClientStreamWithParams", create, func() []interface{} {
		if createErr != nil {
			return []interface{}{nil, createErr}
		}
		s.opts, _ = create.Args()[len(create.Args())-1].([]grpc.CallOption)
		s.cs = newTestClientStream(testStreamMethod, s.desc)
		s.finished = false
		s.watcher = goroutineContext()
		if started != nil {
			started()
		}
		return []interface{}{s.cs, nil}
	})
}

// goroutineContext returns the tracing context that a goroutine started now gets.
func goroutineContext() interface{} {
	if gls, ok := core.GetGLS().(core.ContextSnapshoter); ok {
		return gls.TakeSnapShot()
	}
	return nil
}

// finish replays clientStream.finish: only the first call finishes the
// clientStream, and it calls the OnFinish call options with the final status,
// where io.EOF is a success.
func (s *testStream) finish(cs *nativeclientStream, err error) {
	if cs != s.cs || s.finished {
		return
	}
	s.finished = true
	if err == io.EOF {
		err = nil
	}
	callOnFinish(s.opts, err)
}

// callOnFinish calls the OnFinish call options, as grpc does when a stream
// finishes.
func callOnFinish(opts []grpc.CallOption, err error) {
	for _, opt := range opts {
		if onFinish, ok := opt.(grpc.OnFinishCallOption); ok {
			onFinish.OnFinish(err)
		}
	}
}

func (s *testStream) sendMsg(err error) {
	hooked(s.t, "*clientStream", "SendMsg", operator.NewInvocation(s.cs, struct{}{}), func() []interface{} {
		if err != nil && err != io.EOF {
			s.finish(s.cs, err)
		}
		return []interface{}{err}
	})
}

func (s *testStream) closeSend() {
	hooked(s.t, "*clientStream", "CloseSend", operator.NewInvocation(s.cs), func() []interface{} {
		return []interface{}{nil}
	})
}

func (s *testStream) recvMsg(err error) {
	hooked(s.t, "*clientStream", "RecvMsg", operator.NewInvocation(s.cs, struct{}{}), func() []interface{} {
		if err != nil {
			s.finish(s.cs, err)
		}
		return []interface{}{err}
	})
}

// contextDone replays the watcher goroutine when the stream context is
// canceled or the ClientConn is closed.
func (s *testStream) contextDone(err error) {
	saved := core.GetGLS()
	core.SetGLS(s.watcher)
	defer core.SetGLS(saved)
	s.finish(s.cs, err)
}

func startEntrySpan(t *testing.T) tracing.Span {
	t.Helper()
	entry, err := tracing.CreateEntrySpan(testEntryOperation, func(headerKey string) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

// waitSegment waits until the segment is reported, and checks it has the
// expected number of spans and is reported once.
func waitSegment(t *testing.T, count int) []reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < count && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	spans := core.GetReportedSpans()
	if len(spans) != count {
		names := make([]string, 0, len(spans))
		for _, span := range spans {
			names = append(names, span.OperationName())
		}
		t.Fatalf("reported spans = %d %v, want %d", len(spans), names, count)
	}
	return spans
}

func countSpans(spans []reporter.ReportedSpan, operationName string) int {
	count := 0
	for _, span := range spans {
		if span.OperationName() == operationName {
			count++
		}
	}
	return count
}

func checkExitSpan(t *testing.T, spans []reporter.ReportedSpan, wantError bool) reporter.ReportedSpan {
	t.Helper()
	exit := findReportedSpan(t, spans, testStreamOperation)
	entry := findReportedSpan(t, spans, testEntryOperation)
	if exit.Context().GetParentSpanID() != entry.Context().GetSpanID() {
		t.Fatalf("exit span parent = %d, want the entry span %d", exit.Context().GetParentSpanID(), entry.Context().GetSpanID())
	}
	if exit.IsError() != wantError {
		t.Fatalf("exit span error = %v, want %v", exit.IsError(), wantError)
	}
	return exit
}

func checkChildOf(t *testing.T, spans []reporter.ReportedSpan, operationName string, parent reporter.ReportedSpan) {
	t.Helper()
	span := findReportedSpan(t, spans, operationName)
	if span.Context().GetParentSpanID() != parent.Context().GetSpanID() {
		t.Fatalf("%s parent = %d, want %d", operationName, span.Context().GetParentSpanID(), parent.Context().GetSpanID())
	}
}

// TestServerStreamingSpans pins the spans of a server-streaming call that
// reads until io.EOF, which the grpc scenario expects. clientStreamWrapper
// calls CloseSend after the single SendMsg.
func TestServerStreamingSpans(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s, _, err := newTestStream(t, &nativeStreamDesc{ServerStreams: true}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.sendMsg(nil)
	s.closeSend()
	s.recvMsg(nil)
	s.recvMsg(nil)
	s.recvMsg(io.EOF)
	entry.End()

	spans := waitSegment(t, 7)
	exit := checkExitSpan(t, spans, false)
	checkChildOf(t, spans, testStreamOperation+"/Client/Request/SendMsg", exit)
	checkChildOf(t, spans, testStreamOperation+"/Client/Response/CloseSend", exit)
	checkChildOf(t, spans, testStreamOperation+"/Client/Response/CloseRecv", exit)
	if got := countSpans(spans, testStreamOperation+"/Client/Response/RecvMsg"); got != 2 {
		t.Fatalf("RecvMsg spans = %d, want 2", got)
	}
}

// TestClientStreamingSpans pins the spans of a client-streaming CloseAndRecv,
// where clientStreamWrapper reads the response and then the trailer, which
// finishes the stream. The grpc scenario expects a single CloseRecv span.
func TestClientStreamingSpans(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s, _, err := newTestStream(t, &nativeStreamDesc{ClientStreams: true}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.sendMsg(nil)
	s.sendMsg(nil)
	s.closeSend()
	s.recvMsg(nil)
	s.recvMsg(io.EOF)
	entry.End()

	spans := waitSegment(t, 6)
	exit := checkExitSpan(t, spans, false)
	checkChildOf(t, spans, testStreamOperation+"/Client/Response/CloseRecv", exit)
	if got := countSpans(spans, testStreamOperation+"/Client/Request/SendMsg"); got != 2 {
		t.Fatalf("SendMsg spans = %d, want 2", got)
	}
}

// TestStreamCreationErrorEndsExitSpan covers a NewStream that fails: the exit
// span must end with the error, and the segment must be reported.
func TestStreamCreationErrorEndsExitSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	_, stream, err := newTestStream(t, &nativeStreamDesc{ServerStreams: true}, errors.New("rpc error: code = Unavailable"), nil)
	if err == nil || stream != nil {
		t.Fatalf("NewStream = %v, %v, want an error", stream, err)
	}
	entry.End()

	checkExitSpan(t, waitSegment(t, 2), true)
}

// TestWrappedStreamFinishesExitSpan covers a user stream interceptor that wraps
// the stream: the exit span must still follow the clientStream behind it.
func TestWrappedStreamFinishesExitSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s, stream, err := newTestStream(t, &nativeStreamDesc{ServerStreams: true}, nil, func(s *testStream, stream interface{}) interface{} {
		return &testWrappedStream{inner: stream}
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stream.(*testWrappedStream); !ok {
		t.Fatalf("NewStream returned %T, want the wrapped stream", stream)
	}
	s.sendMsg(nil)
	s.closeSend()
	s.recvMsg(nil)
	s.recvMsg(nil)
	s.recvMsg(io.EOF)
	entry.End()

	spans := waitSegment(t, 7)
	exit := checkExitSpan(t, spans, false)
	checkChildOf(t, spans, testStreamOperation+"/Client/Response/CloseRecv", exit)
}

// TestCancelledStreamFinishesExitSpan covers a stream whose context is
// canceled before any RecvMsg: grpc finishes it from its watcher goroutine.
func TestCancelledStreamFinishesExitSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s, _, err := newTestStream(t, &nativeStreamDesc{ServerStreams: true}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.sendMsg(nil)
	s.closeSend()
	s.contextDone(context.Canceled)
	entry.End()

	spans := waitSegment(t, 4)
	exit := checkExitSpan(t, spans, true)
	checkChildOf(t, spans, testStreamOperation+"/Client/Request/SendMsg", exit)
	checkChildOf(t, spans, testStreamOperation+"/Client/Response/CloseSend", exit)
}

// TestSendErrorFinishesExitSpan covers a SendMsg that fails before any RecvMsg,
// for example with a message larger than the limit: SendMsg finishes the stream.
func TestSendErrorFinishesExitSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s, _, err := newTestStream(t, &nativeStreamDesc{ClientStreams: true}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.sendMsg(errors.New("rpc error: code = ResourceExhausted"))
	entry.End()

	spans := waitSegment(t, 3)
	exit := checkExitSpan(t, spans, true)
	send := findReportedSpan(t, spans, testStreamOperation+"/Client/Request/SendMsg")
	if !send.IsError() || send.Context().GetParentSpanID() != exit.Context().GetSpanID() {
		t.Fatal("the SendMsg span must be an error span under the exit span")
	}
}

// TestStreamFinishedBeforeNewStreamReturns covers a stream that grpc finishes
// before ClientConn.NewStream returns, for example when a stream interceptor
// cancels it: the exit span must end once, after NewStream returned.
func TestStreamFinishedBeforeNewStreamReturns(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	_, _, err := newTestStream(t, &nativeStreamDesc{ServerStreams: true}, nil, func(s *testStream, stream interface{}) interface{} {
		s.contextDone(context.Canceled)
		return stream
	})
	if err != nil {
		t.Fatal(err)
	}
	entry.End()

	checkExitSpan(t, waitSegment(t, 2), true)
}

// TestStreamFinishesExitSpanOnce finishes the stream several times: on the
// failed RecvMsg, from the watcher goroutine, and on a later RecvMsg.
func TestStreamFinishesExitSpanOnce(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s, _, err := newTestStream(t, &nativeStreamDesc{ServerStreams: true}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.recvMsg(errors.New("rpc error: code = Unavailable"))
	s.contextDone(context.Canceled)
	s.recvMsg(errors.New("rpc error: code = Unavailable"))
	entry.End()

	spans := waitSegment(t, 4)
	checkExitSpan(t, spans, true)
	if got := countSpans(spans, testStreamOperation+"/Client/Response/CloseRecv"); got != 2 {
		t.Fatalf("CloseRecv spans = %d, want 2", got)
	}
}

// TestStreamInterceptorRPCKeepsItsClientStream covers a stream interceptor that
// makes a unary RPC with the stream context before it creates the stream: only
// the clientStream of the stream may finish the stream span.
func TestStreamInterceptorRPCKeepsItsClientStream(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s := &testStream{t: t, desc: &nativeStreamDesc{ServerStreams: true}}
	unary := newTestClientStream(testUnaryMethod, &nativeStreamDesc{})
	var unaryOpts []grpc.CallOption
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), s.desc, testStreamMethod, []grpc.CallOption(nil))
	hooked(t, "*ClientConn", "NewStream", newStream, func() []interface{} {
		ctx := newStream.Args()[0].(context.Context)
		createUnary := operator.NewInvocation(nil, ctx, unary.desc, &nativeClientConn{}, testUnaryMethod,
			nil, func() {}, false, []grpc.CallOption(nil))
		hooked(t, "", "newClientStreamWithParams", createUnary, func() []interface{} {
			unaryOpts, _ = createUnary.Args()[len(createUnary.Args())-1].([]grpc.CallOption)
			return []interface{}{unary, nil}
		})
		created := s.create(ctx, nil, nil)
		return []interface{}{&testClientStreamWrapper{clientStream: created[0]}, nil}
	})
	if clientStreamContextData(unary) != nil || len(unaryOpts) != 0 {
		t.Fatal("the clientStream of the unary RPC must not take the stream span")
	}
	s.recvMsg(io.EOF)
	entry.End()

	spans := waitSegment(t, 3)
	exit := checkExitSpan(t, spans, false)
	checkChildOf(t, spans, testStreamOperation+"/Client/Response/CloseRecv", exit)
}

// TestCancelBeforeClientStreamAttached covers a stream context canceled while
// newClientStreamWithParams runs: grpc starts its watcher goroutine before
// newClientStreamWithParams returns, so the watcher can finish the clientStream
// before the agent attaches anything to it.
func TestCancelBeforeClientStreamAttached(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s := &testStream{t: t, desc: &nativeStreamDesc{ServerStreams: true}}
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), s.desc, testStreamMethod, []grpc.CallOption(nil))
	hooked(t, "*ClientConn", "NewStream", newStream, func() []interface{} {
		created := s.create(newStream.Args()[0].(context.Context), nil, func() {
			s.contextDone(context.Canceled)
		})
		return []interface{}{&testClientStreamWrapper{clientStream: created[0]}, nil}
	})
	entry.End()

	checkExitSpan(t, waitSegment(t, 2), true)
}

// TestStreamInterceptorPanicEndsExitSpan covers a stream interceptor that
// creates the stream and then panics: NewStream does not return the stream,
// so the exit span must end then, and a later finish of the abandoned
// clientStream must not end it again.
func TestStreamInterceptorPanicEndsExitSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s := &testStream{t: t, desc: &nativeStreamDesc{ServerStreams: true}}
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), s.desc, testStreamMethod, []grpc.CallOption(nil))
	interceptor := &ClientStreamingInterceptor{}
	func() {
		defer func() {
			if r := recover(); r != "interceptor panic" {
				t.Fatalf("recovered %v, want the interceptor panic", r)
			}
		}()
		runInterceptor(t, "NewStream", func() error { return interceptor.BeforeInvoke(newStream) })
		// the agent defers the after-hook, which sees the zero results while the panic unwinds
		defer runInterceptor(t, "NewStream", func() error { return interceptor.AfterInvoke(newStream, nil, nil) })
		s.create(newStream.Args()[0].(context.Context), nil, nil)
		panic("interceptor panic")
	}()
	entry.End()
	s.contextDone(context.Canceled)

	checkExitSpan(t, waitSegment(t, 2), true)
}

// TestStreamCreationRetriedByInterceptor covers a stream interceptor that calls
// the streamer again after a failed creation: the clientStream of the second
// creation must finish the exit span.
func TestStreamCreationRetriedByInterceptor(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s := &testStream{t: t, desc: &nativeStreamDesc{ServerStreams: true}}
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), s.desc, testStreamMethod, []grpc.CallOption(nil))
	hooked(t, "*ClientConn", "NewStream", newStream, func() []interface{} {
		ctx := newStream.Args()[0].(context.Context)
		if failed := s.create(ctx, errors.New("rpc error: code = Unavailable"), nil); failed[1] == nil {
			t.Fatal("the first creation must fail")
		}
		created := s.create(ctx, nil, nil)
		return []interface{}{&testClientStreamWrapper{clientStream: created[0]}, nil}
	})
	s.recvMsg(io.EOF)
	entry.End()

	spans := waitSegment(t, 3)
	exit := checkExitSpan(t, spans, false)
	checkChildOf(t, spans, testStreamOperation+"/Client/Response/CloseRecv", exit)
}

// TestPanickingUserOnFinishFinishesExitSpan covers an OnFinish call option of
// the application that panics when the stream finishes: grpc calls the
// options without recovering, so the exit span must still finish.
func TestPanickingUserOnFinishFinishesExitSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := startEntrySpan(t)
	s := &testStream{t: t, desc: &nativeStreamDesc{ServerStreams: true}, userOpts: []grpc.CallOption{
		grpc.OnFinish(func(error) { panic("user OnFinish") }),
	}}
	newStream := operator.NewInvocation(&nativeClientConn{}, context.Background(), s.desc, testStreamMethod, []grpc.CallOption(nil))
	hooked(t, "*ClientConn", "NewStream", newStream, func() []interface{} {
		created := s.create(newStream.Args()[0].(context.Context), nil, nil)
		return []interface{}{&testClientStreamWrapper{clientStream: created[0]}, nil}
	})
	func() {
		defer func() {
			if r := recover(); r != "user OnFinish" {
				t.Fatalf("recovered %v, want the user OnFinish panic", r)
			}
		}()
		s.contextDone(context.Canceled)
	}()
	entry.End()

	checkExitSpan(t, waitSegment(t, 2), true)
}
