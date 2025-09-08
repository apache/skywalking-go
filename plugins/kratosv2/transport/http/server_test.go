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
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/reporter"

	"github.com/go-kratos/examples/helloworld/helloworld"
	"github.com/go-kratos/kratos/v2/transport/http"

	"github.com/stretchr/testify/assert"

	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
)

func init() {
	core.ResetTracingContext()
}

type service struct {
	helloworld.UnimplementedGreeterServer
}

func (s *service) SayHello(ctx context.Context, in *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	return &helloworld.HelloReply{Message: fmt.Sprintf("Hello %s", in.Name)}, nil
}

func TestServer(t *testing.T) {
	defer core.ResetTracingContext()
	// start server
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServer(http.Listener(ln), http.Middleware(serverMiddleware))
	endpoint, err := mux.Endpoint()
	helloworld.RegisterGreeterHTTPServer(mux, &service{})
	assert.Nil(t, err, "mux.Endpoint() error = %v", err)
	assert.NotNil(t, endpoint, "mux.Endpoint() endpoint = %v", endpoint)
	go func() {
		if err1 := mux.Start(context.Background()); err1 != nil {
			panic(err1)
		}
	}()
	defer func() {
		_ = mux.Stop(context.Background())
	}()
	time.Sleep(time.Second)

	// start client
	client, err := http.NewClient(context.Background(), http.WithEndpoint(endpoint.String()), http.WithMiddleware(clientMiddleware))
	assert.Nil(t, err, "http.NewClient() error = %v", err)
	assert.NotNil(t, client, "http.NewClient() client = %v", client)
	res := &helloworld.HelloReply{}
	err = client.Invoke(context.Background(), "GET", "/helloworld/test", nil, &res)
	assert.Nil(t, err, "client.Invoke() error = %v", err)
	defer func() {
		_ = client.Close()
	}()

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.Equal(t, 2, len(spans), "len(spans) = %v", len(spans))
	for _, s := range spans {
		switch s.SpanType() {
		case agentv3.SpanType_Entry:
			verifyEntrySpan(t, s)
		case agentv3.SpanType_Exit:
			verifyExitSpan(t, endpoint, s)
		default:
			t.Errorf("unexpected span type: %s", s.SpanType())
		}
	}
}

func verifyEntrySpan(t *testing.T, span reporter.ReportedSpan) {
	assert.Equal(t, "/helloworld.Greeter/SayHello", span.OperationName(), "entrySpan.OperationName() = %s", span.OperationName())
	assert.Equal(t, agentv3.SpanLayer_RPCFramework, span.SpanLayer(), "entrySpan.SpanLayer() = %s", span.SpanLayer())
	assert.Equal(t, 1, len(span.Refs()), "len(entrySpan.References()) = %d", len(span.Refs()))
	assert.GreaterOrEqual(t, span.EndTime(), span.StartTime(), "exitSpan.StartTime() = %d", span.StartTime())
}

func verifyExitSpan(t *testing.T, endpoint *url.URL, span reporter.ReportedSpan) {
	assert.Equal(t, "/helloworld/test", span.OperationName(), "exitSpan.OperationName() = %s", span.OperationName())
	assert.Equal(t, endpoint.Host, span.Peer(), "exitSpan.Peer() = %s", span.Peer())
	assert.Equal(t, agentv3.SpanLayer_RPCFramework, span.SpanLayer(), "exitSpan.SpanLayer() = %s", span.SpanLayer())
	assert.GreaterOrEqual(t, span.EndTime(), span.StartTime(), "exitSpan.StartTime() = %d", span.StartTime())
}
