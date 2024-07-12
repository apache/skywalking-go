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

package mux

import (
	"fmt"
	"net/http"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ServeHTTPInterceptor struct {
}

func (n *ServeHTTPInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	request := invocation.Args()[1].(*http.Request)
	s, err := tracing.CreateEntrySpan(fmt.Sprintf("%s:%s", request.Method, request.RequestURI), func(headerKey string) (string, error) {
		return request.Header.Get(headerKey), nil
	}, tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithComponent(5017),
		tracing.WithTag(tracing.TagHTTPMethod, request.Method),
		tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path))
	if err != nil {
		return err
	}

	rw := newResponseWriter(invocation.Args()[0])
	invocation.ChangeArg(0, rw)
	invocation.SetContext(s)
	return nil
}

func (n *ServeHTTPInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
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

func newResponseWriter(val interface{}) http.ResponseWriter {
	var rw http.ResponseWriter
	sourceWriter := val.(http.ResponseWriter)
	switch val.(type) {
	case http.Hijacker:
		rw = newWriterWrapperWithHijacker(sourceWriter, sourceWriter.(http.Hijacker))
	default:
		rw = newWriterWrapper(rw)
	}
	return rw
}

func newWriterWrapper(writer http.ResponseWriter) *writerWrapper {
	return &writerWrapper{
		ResponseWriter: writer,
		statusCode:     http.StatusOK,
	}
}

type writerWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *writerWrapper) WriteHeader(statusCode int) {
	// cache the status code
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func newWriterWrapperWithHijacker(writer http.ResponseWriter, hijacker http.Hijacker) *writerWrapperWithHijacker {
	return &writerWrapperWithHijacker{
		ResponseWriter: newWriterWrapper(writer),
		Hijacker:       hijacker,
	}
}

type writerWrapperWithHijacker struct {
	http.ResponseWriter
	http.Hijacker
}
