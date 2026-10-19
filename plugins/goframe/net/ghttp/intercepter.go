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
	invocation.ChangeArg(0, wrapResponseWriter(writer))
	invocation.SetContext(s)
	return nil
}

// AfterInvoke processes after the HTTP request has been handled.
func (h *GoFrameServerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if wrapped, ok := invocation.Args()[0].(statusCodeHolder); ok {
		span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", wrapped.cachedStatusCode()))
	}
	span.End()
	return nil
}

// wrapResponseWriter wraps the writer given to the GoFrame server. The wrapper
// implements http.Flusher and http.Hijacker exactly when the wrapped writer
// does: GoFrame flushes through a checked w.(http.Flusher) and hijacks through
// an unchecked w.(http.Hijacker). Unwrap lets http.ResponseController reach
// every optional feature of the wrapped writer.
func wrapResponseWriter(writer http.ResponseWriter) http.ResponseWriter {
	wrapper := &writerWrapper{ResponseWriter: writer, statusCode: http.StatusOK}
	flusher, isFlusher := writer.(http.Flusher)
	hijacker, isHijacker := writer.(http.Hijacker)
	switch {
	case isFlusher && isHijacker:
		return &flushHijackWriterWrapper{wrapper, flusher, hijacker}
	case isFlusher:
		return &flushWriterWrapper{wrapper, flusher}
	case isHijacker:
		return &hijackWriterWrapper{wrapper, hijacker}
	default:
		return wrapper
	}
}

// statusCodeHolder is implemented by all the writer wrappers.
type statusCodeHolder interface {
	cachedStatusCode() int
}

type writerWrapper struct {
	http.ResponseWriter
	statusCode int
}

// Unwrap returns the wrapped writer, for http.ResponseController.
func (w *writerWrapper) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *writerWrapper) cachedStatusCode() int {
	return w.statusCode
}

type flushWriterWrapper struct {
	*writerWrapper
	http.Flusher
}

// FlushError lets http.ResponseController.Flush return the flush error of the wrapped writer.
func (w *flushWriterWrapper) FlushError() error {
	return http.NewResponseController(w.ResponseWriter).Flush()
}

type hijackWriterWrapper struct {
	*writerWrapper
	http.Hijacker
}

type flushHijackWriterWrapper struct {
	*writerWrapper
	http.Flusher
	http.Hijacker
}

// FlushError lets http.ResponseController.Flush return the flush error of the wrapped writer.
func (w *flushHijackWriterWrapper) FlushError() error {
	return http.NewResponseController(w.ResponseWriter).Flush()
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
