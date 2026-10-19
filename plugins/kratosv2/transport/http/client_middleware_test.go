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

package http

import (
	"context"
	nativehttp "net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/reporter"

	"github.com/go-kratos/examples/helloworld/helloworld"
	"github.com/go-kratos/kratos/v2/transport/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
)

// TestClientMiddlewareNilRequestHeader covers a body-less call that passes
// http.Header(&h) with a nil h and sets no User-Agent. From kratos v2.7.3,
// Invoke then sends the request with a nil header map, which net/http
// accepts. The client middleware must not panic on it, and must still
// propagate the context.
func TestClientMiddlewareNilRequestHeader(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	var (
		mu       sync.Mutex
		received nativehttp.Header
	)
	server := httptest.NewServer(nativehttp.HandlerFunc(func(w nativehttp.ResponseWriter, r *nativehttp.Request) {
		mu.Lock()
		received = r.Header.Clone()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Reply", "reply")
		_, _ = w.Write([]byte(`{"message":"hello"}`))
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)

	client, err := http.NewClient(context.Background(),
		http.WithEndpoint(server.URL), http.WithMiddleware(clientMiddleware))
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	// a nil header: kratos fills it with the reply header after the call
	var header nativehttp.Header
	reply := &helloworld.HelloReply{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("client middleware panicked: %v", r)
			}
		}()
		err = client.Invoke(context.Background(), "GET", "/helloworld/test", nil, reply, http.Header(&header))
	}()
	require.NoError(t, err)
	assert.Equal(t, "hello", reply.Message)
	assert.Equal(t, "reply", header.Get("X-Reply"), "the caller still gets the reply header")

	mu.Lock()
	sw8 := received.Get("sw8")
	mu.Unlock()
	assert.NotEmpty(t, sw8, "the sw8 header is still propagated")

	spans := waitForSpans(t, 1)
	require.Len(t, spans, 1)
	assert.Equal(t, agentv3.SpanType_Exit, spans[0].SpanType())
	assert.Equal(t, "/helloworld/test", spans[0].OperationName())
	assert.Equal(t, endpoint.Host, spans[0].Peer())
	assert.False(t, spans[0].IsError())
}

func waitForSpans(t *testing.T, count int) []reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		spans := core.GetReportedSpans()
		if len(spans) >= count || time.Now().After(deadline) {
			return spans
		}
		time.Sleep(10 * time.Millisecond)
	}
}
