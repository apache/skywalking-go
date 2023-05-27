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
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ServerInterceptor struct {
}

func (h *ServerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	request := invocation.Args()[1].(*http.Request)
	s, err := tracing.CreateEntrySpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), func(headerKey string) (string, error) {
		return request.Header.Get(headerKey), nil
	}, tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, request.Method),
		tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path),
		tracing.WithComponent(5004))
	if err != nil {
		return err
	}
	writer := invocation.Args()[0].(http.ResponseWriter)
	response := &writerWrapper{ResponseWriter: writer, statusCode: http.StatusOK, span: s, hijacked: false}
	invocation.ChangeArg(0, response)
	invocation.SetContext(response)
	return nil
}

func (h *ServerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	wrapper := invocation.GetContext().(*writerWrapper)
	if !wrapper.hijacked {
		wrapper.endSpan()
	}
	return nil
}

type writerWrapper struct {
	http.ResponseWriter
	statusCode int
	span       tracing.Span
	hijacked   bool
}

func (w *writerWrapper) WriteHeader(statusCode int) {
	// cache the status code
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *writerWrapper) Hijack() (rwc net.Conn, buf *bufio.ReadWriter, err error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		// if needs hijack, then wrapper the connection, close the span when the connection is closed
		conn, writer, err := h.Hijack()
		if err == nil && conn != nil {
			w.hijacked = true
			conn = &connectionWrapper{Conn: conn, span: w}
		}
		return conn, writer, err
	}
	return nil, nil, fmt.Errorf("responseWriter does not implement http.Hijacker")
}

type connectionWrapper struct {
	net.Conn
	span *writerWrapper
}

func (c *connectionWrapper) Close() error {
	c.span.endSpan()
	return c.Conn.Close()
}

func (w *writerWrapper) endSpan() {
	defer w.span.End()
	w.span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", w.statusCode))
}
