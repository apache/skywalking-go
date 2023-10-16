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

package fasthttp

import (
	"fmt"
	"github.com/valyala/fasthttp"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ClientInterceptor struct {
}

func (h *ClientInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	request := invocation.Args()[0].(*fasthttp.Request)
	s, err := tracing.CreateExitSpan(fmt.Sprintf("%s:%s", string(request.Header.Method()), request.URI().String()),
		string(request.Host()), func(headerKey, headerValue string) error {
			request.Header.Add(headerKey, headerValue)
			return nil
		}, tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, string(request.Header.Method())),
		tracing.WithTag(tracing.TagURL, request.URI().String()),
		tracing.WithComponent(5014))
	if err != nil {
		return err
	}
	invocation.SetContext(s)
	return nil
}

func (h *ClientInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if resp, ok := invocation.Args()[1].(*fasthttp.Response); ok && resp != nil {
		if resp.StatusCode() >= 400 {
			span.Error(string(resp.Body()))
		}
		span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", resp.StatusCode()))

		if requestId := resp.Header.Peek("X-Bce-Request-Id"); requestId != nil {
			span.Tag(tracing.TagReqId, string(requestId))
		}
	}
	if err, ok := result[0].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
