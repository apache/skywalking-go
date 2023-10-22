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

package fiber

import (
	"fmt"

	"github.com/valyala/fasthttp"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type HTTPInterceptor struct {
}

func (h *HTTPInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	ctx := invocation.Args()[0].(*fasthttp.RequestCtx)
	s, err := tracing.CreateEntrySpan(fmt.Sprintf("%s:%s", string(ctx.Method()), string(ctx.Request.URI().Path())),
		func(headerKey string) (string, error) {
			return string(ctx.Request.Header.Peek(headerKey)), nil
		},
		tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, string(ctx.Method())),
		tracing.WithTag(tracing.TagURL, string(ctx.Request.URI().Host())+string(ctx.Request.URI().Path())),
		tracing.WithComponent(5021))
	if err != nil {
		return err
	}
	invocation.SetContext(s)
	return nil
}

func (h *HTTPInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if ctx, ok := invocation.Args()[0].(*fasthttp.RequestCtx); ok {
		if ctx.Response.StatusCode() >= 400 {
			span.Error()
		}
		span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", ctx.Response.StatusCode()))
	}
	span.End()
	return nil
}
