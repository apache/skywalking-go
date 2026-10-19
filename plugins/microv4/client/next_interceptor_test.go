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

package client

import (
	"testing"
	"time"

	"go-micro.dev/v4/client"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/selector"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

func testNodeSelector() (*registry.Node, error) {
	return &registry.Node{Address: "10.0.0.1:8080"}, nil
}

// TestNextKeepsCallerPeerForUntracedCall covers a call that the client wrapper
// runs untraced inside a traced request, because the request has no service:
// the selector must not set the peer of the caller's span.
func TestNextKeepsCallerPeerForUntracedCall(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	entry, err := tracing.CreateEntrySpan("GET:/consumer", func(string) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}
	untraced := operator.NewInvocation(nil, &fakeRequest{endpoint: "Say.Hello"}, client.CallOptions{})
	if err := (&NextInterceptor{}).AfterInvoke(untraced, selector.Next(testNodeSelector), nil); err != nil {
		t.Fatal(err)
	}
	if untraced.IsContinue() {
		t.Fatal("the selector of an untraced call must not be wrapped")
	}
	entry.End()

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	if len(spans) != 1 || spans[0].Peer() != "" {
		t.Fatalf("the caller's entry span must keep its empty peer, got %d spans", len(spans))
	}
}

// TestNextSetsExitSpanPeer keeps the peer of a traced call: the selector sets
// the selected node as the peer of the exit span.
func TestNextSetsExitSpanPeer(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	exit, err := tracing.CreateExitSpan("greeter.Say.Hello", "greeter", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	traced := operator.NewInvocation(nil, &fakeRequest{service: "greeter", endpoint: "Say.Hello"}, client.CallOptions{})
	if err := (&NextInterceptor{}).AfterInvoke(traced, selector.Next(testNodeSelector), nil); err != nil {
		t.Fatal(err)
	}
	if !traced.IsContinue() {
		t.Fatal("the selector of a traced call must be wrapped")
	}
	exit.End()
}
