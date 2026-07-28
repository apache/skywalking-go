// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with this work for
// additional information regarding copyright ownership. Apache Software
// Foundation (ASF) licenses this file to you under the Apache License, Version
// 2.0 (the "License"); you may not use this file except in compliance with the
// License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type testServerTransportStream struct {
	ctx    context.Context
	method string
}

func (s *testServerTransportStream) Context() context.Context {
	return s.ctx
}

func (s *testServerTransportStream) Method() string {
	return s.method
}

func TestServerHandleStreamAcceptsStreamInterface(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	// gRPC v1.81 passes *transport.ServerStream where older releases passed
	// *transport.Stream. Both expose this public method contract.
	stream := &testServerTransportStream{
		ctx:    context.Background(),
		method: "/api.Echo/ServerStreamingEcho",
	}
	invocation := operator.NewInvocation(nil, nil, stream)
	interceptor := &ServerHandleStreamInterceptor{}

	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if invocation.GetContext() == nil {
		t.Fatal("entry span was not stored in invocation context")
	}
	if err := interceptor.AfterInvoke(invocation); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	spans := core.GetReportedSpans()
	if len(spans) != 1 {
		t.Fatalf("reported spans = %d, want 1", len(spans))
	}
	if got := spans[0].OperationName(); got != "api.Echo.ServerStreamingEcho" {
		t.Fatalf("operation name = %q", got)
	}
}

func TestServerHandleStreamRejectsUnsupportedArgument(t *testing.T) {
	invocation := operator.NewInvocation(nil, nil, struct{}{})
	err := (&ServerHandleStreamInterceptor{}).BeforeInvoke(invocation)
	if err == nil || !strings.Contains(err.Error(), "unsupported grpc server transport stream type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerSendResponseAcceptsStreamInterface(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	root, err := tracing.CreateEntrySpan("api.Echo.UnaryEcho", func(string) (string, error) {
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	stream := &testServerTransportStream{
		ctx:    context.Background(),
		method: "/api.Echo/UnaryEcho",
	}
	invocation := operator.NewInvocation(nil, nil, stream)
	interceptor := &ServerSendResponseInterceptor{}

	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}
	if invocation.GetContext() == nil {
		t.Fatal("send response span was not stored in invocation context")
	}
	if err := interceptor.AfterInvoke(invocation, error(nil)); err != nil {
		t.Fatal(err)
	}
	root.End()

	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	spans := core.GetReportedSpans()
	if len(spans) != 2 {
		t.Fatalf("reported spans = %d, want 2", len(spans))
	}
	found := false
	for _, span := range spans {
		if span.OperationName() == "api.Echo.UnaryEcho/Server/Response/SendResponse" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("send response span was not reported")
	}
}
