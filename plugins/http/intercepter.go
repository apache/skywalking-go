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
	"fmt"
	"net/http"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type Interceptor struct {
}

func (h *Interceptor) BeforeInvoke(invocation *operator.Invocation) error {
	request := invocation.Args[0].(*http.Request)
	s, err := tracing.CreateExitSpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), request.Host, func(headerKey, headerValue string) error {
		request.Header.Add(headerKey, headerValue)
		return nil
	}, tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, request.Method),
		tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path),
		tracing.WithComponent(5005))
	if err != nil {
		return err
	}
	invocation.Context = s
	return nil
}

func (h *Interceptor) AfterInvoke(invocation *operator.Invocation, result ...interface{}) error {
	if invocation.Context == nil {
		return nil
	}
	span := invocation.Context.(tracing.Span)
	if resp, ok := result[0].(*http.Response); ok && resp != nil {
		span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", resp.StatusCode))
	}
	if err, ok := result[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
