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

package zrpc

import (
	"context"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

const gozeroComponent int32 = 5023

type ServerMiddlewareInterceptor struct {
}

// BeforeInvoke intercepts the rpc request before invoking the handler.
func (h *ServerMiddlewareInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	server := invocation.CallerInstance().(*zrpc.RpcServer)
	server.AddUnaryInterceptors(RPCServeInterceptor(invocation))
	return nil
}

// AfterInvoke processes after the rpc request has been handled.
func (h *ServerMiddlewareInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	return nil
}

// RPCServeInterceptor is a grpc server interceptor that creates a new span for each incoming request.
var RPCServeInterceptor = func(invocation operator.Invocation) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if activeSpan := tracing.ActiveSpan(); activeSpan != nil {
			activeSpan.SetOperationName(info.FullMethod)
			activeSpan.SetComponent(gozeroComponent)
			activeSpan.SetSpanLayer(tracing.SpanLayerRPCFramework)
			activeSpan.Tag("framework", "go-zero")
			activeSpan.Tag("gozero.endpoint", info.FullMethod)
			activeSpan.Tag("transport", "gRPC")

			invocation.SetContext(activeSpan)
			reply, handlerErr := handler(ctx, req)
			if handlerErr != nil {
				activeSpan.Error(handlerErr.Error())
			}
			return reply, handlerErr
		}

		span, err := tracing.CreateEntrySpan(info.FullMethod, func(headerKey string) (string, error) {
			md, ok := FromIncomingContext(ctx)
			if !ok {
				return "", nil
			}
			values := md.Get(headerKey)
			if len(values) == 0 || values[0] == "" {
				return "", nil
			}
			return values[0], nil
		}, tracing.WithComponent(gozeroComponent),
			tracing.WithTag("framework", "go-zero"),
			tracing.WithTag("gozero.endpoint", info.FullMethod),
			tracing.WithLayer(tracing.SpanLayerRPCFramework),
			tracing.WithTag("transport", "gRPC"))
		if err != nil {
			return handler(ctx, req)
		}
		defer span.End()
		invocation.SetContext(span)

		reply, err := handler(ctx, req)
		if err != nil {
			span.Error(err.Error())
		}
		return reply, err
	}
}

type mdIncomingKey struct{}
type MD map[string][]string

func (md MD) Get(k string) []string {
	return md[k]
}

func FromIncomingContext(ctx context.Context) (MD, bool) {
	md, ok := ctx.Value(mdIncomingKey{}).(MD)
	if !ok {
		return nil, false
	}
	out := make(MD, len(md))
	for k, v := range md {
		out[k] = v
	}
	return out, true
}
