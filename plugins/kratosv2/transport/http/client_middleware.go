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
	"net/url"

	"github.com/apache/skywalking-go/plugins/core/log"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

var clientMiddleware = func(handler middleware.Handler) middleware.Handler {
	return func(c context.Context, req interface{}) (interface{}, error) {
		if tr, ok := transport.FromClientContext(c); ok {
			peer := tr.Endpoint()
			if parse, _ := url.Parse(tr.Endpoint()); parse != nil {
				peer = parse.Host
			}
			span, err := tracing.CreateExitSpan(tr.Operation(), peer, func(key, value string) error {
				tr.RequestHeader().Add(key, value)
				return nil
			}, tracing.WithComponent(5010),
				tracing.WithLayer(tracing.SpanLayerRPCFramework),
				tracing.WithTag("transport", "HTTP"))
			if err != nil {
				log.Warnf("cannot create exit span: %v", err)
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

type ClientMiddlewareInterceptor struct {
}

func (n *ClientMiddlewareInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	if tracing.GetRuntimeContextValue(ignoreClientMiddlewareKey) != nil {
		tracing.SetRuntimeContextValue(ignoreClientMiddlewareKey, nil)
		return nil
	}
	middlewares := invocation.Args()[0].([]middleware.Middleware)
	middlewares = append(middlewares, clientMiddleware)
	invocation.ChangeArg(0, middlewares)
	invocation.SetContext(true)
	return nil
}

func (n *ClientMiddlewareInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if invocation.GetContext() != true {
		return nil
	}
	opt := results[0].(nativeClientOption)
	// wrapper the client option, and adding the true value to the client to let the interceptor know the client has been enhanced
	var clientOption nativeClientOption = func(client *nativeClientOpts) {
		opt(client)
		var clientOptsRef interface{} = client
		if enhance, ok := clientOptsRef.(operator.EnhancedInstance); ok {
			enhance.SetSkyWalkingDynamicField(true)
		}
	}
	invocation.DefineReturnValues(clientOption)
	return nil
}
