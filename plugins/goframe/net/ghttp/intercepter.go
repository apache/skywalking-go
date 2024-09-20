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

package ghttp

// nolint
import (
	"fmt"
	"net/http"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// GoFrameServerInterceptor is used to intercept and trace HTTP requests.
type GoFrameServerInterceptor struct{}

// BeforeInvoke intercepts the HTTP request before invoking the handler.
func (h *GoFrameServerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	request := invocation.Args()[1].(*http.Request)
	s, err := tracing.CreateEntrySpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), func(headerKey string) (string, error) {
		return request.Header.Get(headerKey), nil
	}, tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, request.Method),
		tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path),
		tracing.WithComponent(5022))
	if err != nil {
		return err
	}

	if config.CollectRequestParameters && request.URL != nil {
		s.Tag(tracing.TagHTTPParams, request.URL.RawQuery)
	}
	if len(config.CollectRequestHeaders) > 0 {
		collectRequestHeaders(s, request.Header)
	}

	writer := invocation.Args()[0].(http.ResponseWriter)
	invocation.ChangeArg(0, &writerWrapper{ResponseWriter: writer, statusCode: http.StatusOK})
	invocation.SetContext(s)
	return nil
}

// AfterInvoke processes after the HTTP request has been handled.
func (h *GoFrameServerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if wrapped, ok := invocation.Args()[0].(*writerWrapper); ok {
		span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", wrapped.statusCode))
	}
	span.End()
	return nil
}

type writerWrapper struct {
	http.ResponseWriter
	statusCode int
}

func collectRequestHeaders(span tracing.Span, requestHeaders http.Header) {
	var headerTagValues []string
	for _, header := range config.CollectRequestHeaders {
		var headerValue = requestHeaders.Get(header)
		if headerValue != "" {
			headerTagValues = append(headerTagValues, header+"="+headerValue)
		}
	}

	if len(headerTagValues) == 0 {
		return
	}

	tagValue := strings.Join(headerTagValues, "\n")
	if len(tagValue) > config.HeaderLengthThreshold {
		maxLen := config.HeaderLengthThreshold
		tagValue = tagValue[:maxLen]
	}

	span.Tag(tracing.TagHTTPHeaders, tagValue)
}
