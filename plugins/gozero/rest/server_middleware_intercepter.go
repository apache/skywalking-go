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

package rest

import (
	"fmt"
	"net/http"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
	"github.com/zeromicro/go-zero/rest"
)

const gozeroComponent int32 = 5023

type ServerMiddlewareInterceptor struct {
}

var SkyWalkingMiddleware rest.Middleware = func(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if activeSpan := tracing.ActiveSpan(); activeSpan != nil {
			activeSpan.SetOperationName(fmt.Sprintf("%s:%s", request.Method, request.URL.Path))
			activeSpan.SetComponent(gozeroComponent)
			activeSpan.SetSpanLayer(tracing.SpanLayerHTTP)
			activeSpan.Tag("framework", "go-zero")
			activeSpan.Tag(tracing.TagHTTPMethod, request.Method)
			activeSpan.Tag(tracing.TagURL, request.Host+request.URL.Path)

			// collect response data
			switch request.Method {
			case http.MethodGet:
				if request.URL.RawQuery != "" {
					activeSpan.Tag(tracing.TagHTTPParams, request.URL.RawQuery)
				}
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				if err := request.ParseForm(); err == nil {
					activeSpan.Tag(tracing.TagHTTPParams, request.Form.Encode())
				}
			}

			activeSpan.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", http.StatusOK))
			next(writer, request)
			return
		}

		s, err := tracing.CreateEntrySpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), func(headerKey string) (string, error) {
			return request.Header.Get(headerKey), nil
		}, tracing.WithLayer(tracing.SpanLayerHTTP),
			tracing.WithTag("framework", "go-zero"),
			tracing.WithTag(tracing.TagHTTPMethod, request.Method),
			tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path),
			tracing.WithComponent(gozeroComponent))
		if err != nil {
			next(writer, request)
			return
		}

		defer s.End()

		// collect response data
		switch request.Method {
		case http.MethodGet:
			if request.URL.RawQuery != "" {
				s.Tag(tracing.TagHTTPParams, request.URL.RawQuery)
			}
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			if err := request.ParseForm(); err == nil {
				s.Tag(tracing.TagHTTPParams, request.Form.Encode())
			}
		}
		s.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", http.StatusOK))
		next(writer, request)
	}
}

// BeforeInvoke intercepts the HTTP request before invoking the handler.
func (h *ServerMiddlewareInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	server := invocation.CallerInstance().(*rest.Server)
	server.Use(SkyWalkingMiddleware)
	return nil
}

// AfterInvoke processes after the HTTP request has been handled.
func (h *ServerMiddlewareInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}
