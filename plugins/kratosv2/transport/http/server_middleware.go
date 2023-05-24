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

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"

	"github.com/apache/skywalking-go/plugins/core/log"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

var serverMiddleware = func(handler middleware.Handler) middleware.Handler {
	return func(c context.Context, req interface{}) (interface{}, error) {
		if tr, ok := transport.FromServerContext(c); ok {
			span, err := tracing.CreateEntrySpan(tr.Operation(), func(key string) (string, error) {
				return tr.RequestHeader().Get(key), nil
			}, tracing.WithComponent(5010),
				tracing.WithLayer(tracing.SpanLayerRPCFramework),
				tracing.WithTag("transport", "HTTP"))
			if err != nil {
				log.Warnf("cannot create entry span: %v", err)
				return handler(c, req)
			}
			defer span.End()

			reply, err := handler(c, req)
			if err != nil {
				span.Error(err.Error())
			}
			return reply, err
		}
		return handler(c, req)
	}
}

type ServerMiddlewareInterceptor struct {
}

func (n *ServerMiddlewareInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	if tracing.GetRuntimeContextValue(ignoreServerMiddlewareKey) != nil {
		tracing.SetRuntimeContextValue(ignoreServerMiddlewareKey, nil)
		return nil
	}
	middlewares := invocation.Args()[0].([]middleware.Middleware)
	middlewares = append(middlewares, serverMiddleware)
	invocation.ChangeArg(0, middlewares)
	invocation.SetContext(true)
	return nil
}

func (n *ServerMiddlewareInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if invocation.GetContext() != true {
		return nil
	}
	opt := results[0].(http.ServerOption)
	// wrapper the server option, and adding the true value to the server to let the interceptor know the server has been enhanced
	var serverOption http.ServerOption = func(server *http.Server) {
		opt(server)
		var serverRef interface{} = server
		if enhance, ok := serverRef.(operator.EnhancedInstance); ok {
			enhance.SetSkyWalkingDynamicField(true)
		}
	}
	invocation.DefineReturnValues(serverOption)
	return nil
}
