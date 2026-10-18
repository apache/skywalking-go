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
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const testUnaryMethod = "/api.Echo/UnaryEcho"

func waitReportedSpans(t *testing.T, count int) []reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < count && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	spans := core.GetReportedSpans()
	if len(spans) != count {
		t.Fatalf("reported spans = %d, want %d", len(spans), count)
	}
	return spans
}

func findReportedSpan(t *testing.T, spans []reporter.ReportedSpan, operationName string) reporter.ReportedSpan {
	t.Helper()
	for _, span := range spans {
		if span.OperationName() == operationName {
			return span
		}
	}
	t.Fatalf("span %q was not reported", operationName)
	return nil
}

func spanTagValue(span reporter.ReportedSpan, key string) (string, bool) {
	for _, tag := range span.Tags() {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

func createServerEntrySpan(t *testing.T) tracing.Span {
	t.Helper()
	span, err := tracing.CreateEntrySpan("api.Echo.UnaryEcho", func(string) (string, error) {
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return span
}

func TestServerProcessRPCTagsRPCType(t *testing.T) {
	tests := []struct {
		name string
		desc *nativeStreamDesc
		want string
	}{
		{name: "unary", desc: &nativeStreamDesc{}, want: "Unary"},
		{name: "server streaming", desc: &nativeStreamDesc{ServerStreams: true}, want: "Streaming"},
		{name: "client streaming", desc: &nativeStreamDesc{ClientStreams: true}, want: "Streaming"},
		{name: "bidirectional streaming", desc: &nativeStreamDesc{ClientStreams: true, ServerStreams: true}, want: "Streaming"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core.ResetTracingContext()
			defer core.ResetTracingContext()

			entry := createServerEntrySpan(t)
			// the arguments of processRPC are ctx, stream, info, sd and trInfo
			invocation := operator.NewInvocation(nil, context.Background(), nil, nil, tt.desc, nil)
			interceptor := &ServerProcessRPCInterceptor{}
			if err := interceptor.BeforeInvoke(invocation); err != nil {
				t.Fatal(err)
			}
			if err := interceptor.AfterInvoke(invocation, error(nil)); err != nil {
				t.Fatal(err)
			}
			entry.End()

			spans := waitReportedSpans(t, 1)
			if got, ok := spanTagValue(spans[0], RPCTypeTag); !ok || got != tt.want {
				t.Fatalf("%s tag = %q (present: %v), want %q", RPCTypeTag, got, ok, tt.want)
			}
		})
	}
}

func TestServerProcessRPCWithoutActiveSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	invocation := operator.NewInvocation(nil, context.Background(), nil, nil, &nativeStreamDesc{}, nil)
	if err := (&ServerProcessRPCInterceptor{}).BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
}

func newTestServerStream(method string, desc *nativeStreamDesc) *nativeserverStream {
	return &nativeserverStream{
		s:    &nativeStream{ctx: context.Background(), method: method},
		desc: desc,
	}
}

func TestServerUnarySendMsgCreatesSendResponseSpan(t *testing.T) {
	for _, sendErr := range []error{nil, errors.New("transport is closing")} {
		core.ResetTracingContext()

		entry := createServerEntrySpan(t)
		invocation := operator.NewInvocation(newTestServerStream(testUnaryMethod, &nativeStreamDesc{}), struct{}{})
		interceptor := &ServerSendMsgInterceptor{}
		if err := interceptor.BeforeInvoke(invocation); err != nil {
			t.Fatal(err)
		}
		if err := interceptor.AfterInvoke(invocation, sendErr); err != nil {
			t.Fatal(err)
		}
		entry.End()

		spans := waitReportedSpans(t, 2)
		span := findReportedSpan(t, spans, "api.Echo.UnaryEcho/Server/Response/SendResponse")
		if span.ComponentID() != 23 {
			t.Fatalf("component = %d, want 23", span.ComponentID())
		}
		if span.SpanLayer().String() != "RPCFramework" {
			t.Fatalf("layer = %s, want RPCFramework", span.SpanLayer())
		}
		if got, _ := spanTagValue(span, tracing.TagURL); got != testUnaryMethod {
			t.Fatalf("url tag = %q", got)
		}
		if span.IsError() != (sendErr != nil) {
			t.Fatalf("isError = %v for send error %v", span.IsError(), sendErr)
		}
	}
	core.ResetTracingContext()
}

func TestServerUnarySendMsgWithoutActiveSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	invocation := operator.NewInvocation(newTestServerStream(testUnaryMethod, &nativeStreamDesc{}), struct{}{})
	if err := (&ServerSendMsgInterceptor{}).BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if invocation.GetContext() != nil {
		t.Fatal("unary SendMsg without an entry span must not create a span, like sendResponse")
	}
}

func TestServerStreamingSendMsgKeepsSendMsgSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := createServerEntrySpan(t)
	stream := newTestServerStream("/api.Echo/ServerStreamingEcho", &nativeStreamDesc{ServerStreams: true})
	invocation := operator.NewInvocation(stream, struct{}{})
	interceptor := &ServerSendMsgInterceptor{}
	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if err := interceptor.AfterInvoke(invocation, error(nil)); err != nil {
		t.Fatal(err)
	}
	entry.End()

	findReportedSpan(t, waitReportedSpans(t, 2), "api.Echo.ServerStreamingEcho/Server/Request/SendMsg")
}

func TestServerRecvMsgSkipsUnary(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := createServerEntrySpan(t)
	defer entry.End()
	invocation := operator.NewInvocation(newTestServerStream(testUnaryMethod, &nativeStreamDesc{}), struct{}{})
	interceptor := &ServerRecvMsgInterceptor{}
	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if invocation.GetContext() != nil {
		t.Fatal("unary RecvMsg must not create a span, grpc < v1.84.0 did not read the request through it")
	}
	if err := interceptor.AfterInvoke(invocation, error(nil)); err != nil {
		t.Fatal(err)
	}
}

func TestServerRecvMsgKeepsStreamingSpan(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry := createServerEntrySpan(t)
	defer entry.End()
	stream := newTestServerStream("/api.Echo/ClientStreamingEcho", &nativeStreamDesc{ClientStreams: true})
	invocation := operator.NewInvocation(stream, struct{}{})
	interceptor := &ServerRecvMsgInterceptor{}
	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if invocation.GetContext() == nil {
		t.Fatal("streaming RecvMsg must create a span")
	}
	if err := interceptor.AfterInvoke(invocation, error(nil)); err != nil {
		t.Fatal(err)
	}
}
