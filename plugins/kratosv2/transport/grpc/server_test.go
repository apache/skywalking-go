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

package grpc

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/reporter"

	"github.com/go-kratos/examples/helloworld/helloworld"

	"github.com/go-kratos/kratos/v2/transport/grpc"

	"github.com/stretchr/testify/assert"

	nativeGRPC "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
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

func TestAfterInvoke(t *testing.T) {
	defer core.ResetTracingContext()
	// build server with middleware
	server := grpc.NewServer(grpc.Middleware(serverMiddleware))
	endpoint, err := server.Endpoint()
	assert.Nil(t, err, "server.Endpoint() error = %v", err)
	assert.NotNil(t, endpoint, "server.Endpoint() endpoint is nil")
	helloworld.RegisterGreeterServer(server, &service{})
	go func() {
		// start server
		if err1 := server.Start(context.Background()); err1 != nil {
			assert.Nil(t, err1, "server.Start() error = %v", err1)
		}
	}()
	time.Sleep(time.Second)
	defer func() {
		_ = server.Stop(context.Background())
	}()

	// build client with middleware
	client, err := grpc.Dial(context.Background(),
		grpc.WithEndpoint(endpoint.Host),
		grpc.WithOptions(nativeGRPC.WithTransportCredentials(insecure.NewCredentials())),
		grpc.WithMiddleware(clientMiddleware))
	assert.Nil(t, err, "grpc.Dial() error = %v", err)
	assert.NotNil(t, client, "grpc.Dial() client is nil")
	greeterClient := helloworld.NewGreeterClient(client)
	defer func() {
		_ = client.Close()
	}()

	// send request
	resp, err := greeterClient.SayHello(context.Background(), &helloworld.HelloRequest{Name: "kratos"})
	assert.Nil(t, err, "greeterClient.SayHello() error = %v", err)
	assert.NotNil(t, resp, "greeterClient.SayHello() resp is nil")
	time.Sleep(100 * time.Millisecond)

	spans := core.GetReportedSpans()
	assert.Equal(t, 2, len(spans), "len(spans) = %d", len(spans))
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
	assert.Equal(t, "/helloworld.Greeter/SayHello", span.OperationName(), "exitSpan.OperationName() = %s", span.OperationName())
	assert.Equal(t, endpoint.Host, span.Peer(), "exitSpan.Peer() = %s", span.Peer())
	assert.Equal(t, agentv3.SpanLayer_RPCFramework, span.SpanLayer(), "exitSpan.SpanLayer() = %s", span.SpanLayer())
	assert.GreaterOrEqual(t, span.EndTime(), span.StartTime(), "exitSpan.StartTime() = %d", span.StartTime())
}
