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

package gin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/gin-gonic/gin"
)

type ContextInterceptor struct {
}

func (h *ContextInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	if !isFirstHandle(invocation.CallerInstance()) {
		return nil
	}
	context := invocation.CallerInstance().(*gin.Context)
	fullPath := context.FullPath()
	if fullPath == "" {
		fullPath = context.Request.URL.Path
	}
	s, err := tracing.CreateEntrySpan(
		fmt.Sprintf("%s:%s", context.Request.Method, fullPath), func(headerKey string) (string, error) {
			return context.Request.Header.Get(headerKey), nil
		},
		tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, context.Request.Method),
		tracing.WithTag(tracing.TagURL, context.Request.Host+context.Request.URL.Path),
		tracing.WithComponent(5006))
	if err != nil {
		return err
	}

	if len(config.CollectRequestHeaders) > 0 {
		collectRequestHeaders(s, context.Request.Header)
	}

	invocation.SetContext(s)
	return nil
}

func (h *ContextInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	context := invocation.CallerInstance().(*gin.Context)
	span := invocation.GetContext().(tracing.Span)
	span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", context.Writer.Status()))
	if len(context.Errors) > 0 {
		span.Error(context.Errors.String())
	}
	span.End()
	return nil
}

func isFirstHandle(c interface{}) bool {
	// index of HandlersChain, incremented in #Next(), -1 indicates that no handler has been executed
	if context, ok := c.(*nativeContext); ok {
		return context.index < 0
	}
	return true
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
