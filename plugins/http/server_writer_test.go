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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newInterceptedServer serves handler on a real net/http server, behind the
// ServeMux interceptor, the way the instrumented ServeMux.ServeHTTP runs it.
func newInterceptedServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		interceptor := &ServerInterceptor{}
		invocation := operator.NewInvocation(nil, w, r)
		if err := interceptor.BeforeInvoke(invocation); err != nil {
			t.Errorf("before invoke: %v", err)
			return
		}
		defer func() {
			if err := interceptor.AfterInvoke(invocation); err != nil {
				t.Errorf("after invoke: %v", err)
			}
		}()
		handler(invocation.Args()[0].(http.ResponseWriter), r)
	}))
}

func waitForSpans(t *testing.T, count int) []reporter.ReportedSpan {
	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < count {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d reported spans, got %d", count, len(core.GetReportedSpans()))
		}
		time.Sleep(10 * time.Millisecond)
	}
	return core.GetReportedSpans()
}

func tagValue(span reporter.ReportedSpan, key string) string {
	for _, tag := range span.Tags() {
		if tag.Key == key {
			return tag.Value
		}
	}
	return ""
}

// TestServerWriterKeepsOptionalInterfaces checks that a handler sees the same
// optional features through the wrapped writer as through the server's own
// writer: http.Flusher and http.Hijacker by type assertion, and everything
// http.ResponseController reaches through Unwrap.
func TestServerWriterKeepsOptionalInterfaces(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	server := newInterceptedServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, isFlusher := w.(http.Flusher)
		_, isHijacker := w.(http.Hijacker)
		controller := http.NewResponseController(w)
		deadlineErr := controller.SetWriteDeadline(time.Now().Add(time.Minute))
		duplexErr := controller.EnableFullDuplex()
		w.WriteHeader(http.StatusAccepted)
		flushErr := controller.Flush()
		_, _ = fmt.Fprintf(w, "flusher=%v hijacker=%v deadline=%v duplex=%v flush=%v",
			isFlusher, isHijacker, deadlineErr, duplexErr, flushErr)
	})
	defer server.Close()

	resp, err := http.Get(server.URL + "/features")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, "flusher=true hijacker=true deadline=<nil> duplex=<nil> flush=<nil>", string(body))
	spans := waitForSpans(t, 1)
	assert.Equal(t, "202", tagValue(spans[0], tracing.TagStatusCode), "the status code must still be captured")
}

// TestServerWriterFlushReachesClient is the server-sent events case: the first
// event must reach the client while the handler is still running.
func TestServerWriterFlushReachesClient(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	received := make(chan struct{})
	server := newInterceptedServer(t, func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: ping\n\n")
		flusher.Flush()
		select {
		case <-received:
			_, _ = io.WriteString(w, "data: done\n\n")
		case <-time.After(5 * time.Second):
			_, _ = io.WriteString(w, "data: timeout\n\n")
		}
	})
	defer server.Close()

	client := &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 2 * time.Second}}
	resp, err := client.Get(server.URL + "/events")
	require.NoError(t, err, "the response headers must be flushed before the handler returns")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	reader := bufio.NewReader(resp.Body)
	first, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "data: ping\n", first)
	close(received)
	rest, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "\ndata: done\n\n", string(rest))
}

// plainWriter implements only http.ResponseWriter.
type plainWriter struct {
	header http.Header
	status int
}

func (p *plainWriter) Header() http.Header         { return p.header }
func (p *plainWriter) Write(b []byte) (int, error) { return len(b), nil }
func (p *plainWriter) WriteHeader(status int)      { p.status = status }

func wrapWithInterceptor(t *testing.T, writer http.ResponseWriter) (http.ResponseWriter, operator.Invocation) {
	request, err := http.NewRequest(http.MethodGet, "http://localhost/", http.NoBody)
	require.NoError(t, err)
	invocation := operator.NewInvocation(nil, writer, request)
	require.NoError(t, (&ServerInterceptor{}).BeforeInvoke(invocation))
	return invocation.Args()[0].(http.ResponseWriter), invocation
}

// TestServerWriterDoesNotAdvertiseMissingInterfaces checks that the wrapper
// exposes an optional interface only when the wrapped writer implements it, so
// that handlers take the same branch as without the agent, and that
// http.ResponseController reports http.ErrNotSupported for the missing ones.
func TestServerWriterDoesNotAdvertiseMissingInterfaces(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	// like the HTTP/2 server writer, the recorder can flush but not hijack
	recorder := httptest.NewRecorder()
	wrapped, invocation := wrapWithInterceptor(t, recorder)
	_, isHijacker := wrapped.(http.Hijacker)
	assert.False(t, isHijacker, "the wrapped writer cannot hijack")
	_, _, err := http.NewResponseController(wrapped).Hijack()
	assert.True(t, errors.Is(err, http.ErrNotSupported), "hijack error should be ErrNotSupported, got %v", err)
	flusher, isFlusher := wrapped.(http.Flusher)
	require.True(t, isFlusher, "the wrapped writer can flush")
	flusher.Flush()
	assert.True(t, recorder.Flushed, "flush must reach the wrapped writer")
	require.NoError(t, (&ServerInterceptor{}).AfterInvoke(invocation))

	plain := &plainWriter{header: http.Header{}}
	wrapped, invocation = wrapWithInterceptor(t, plain)
	_, isHijacker = wrapped.(http.Hijacker)
	_, isFlusher = wrapped.(http.Flusher)
	assert.False(t, isHijacker, "the wrapped writer cannot hijack")
	assert.False(t, isFlusher, "the wrapped writer cannot flush")
	assert.True(t, errors.Is(http.NewResponseController(wrapped).Flush(), http.ErrNotSupported))
	wrapped.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, plain.status)
	require.NoError(t, (&ServerInterceptor{}).AfterInvoke(invocation))

	statusCodes := map[string]bool{}
	for _, span := range waitForSpans(t, 2) {
		statusCodes[tagValue(span, tracing.TagStatusCode)] = span.IsError()
	}
	assert.Equal(t, map[string]bool{"200": false, "404": true}, statusCodes)
}

// flushErrorWriter fails to flush, like a server writer whose client is gone.
type flushErrorWriter struct {
	plainWriter
	err error
}

func (f *flushErrorWriter) Flush()            {}
func (f *flushErrorWriter) FlushError() error { return f.err }

// TestServerWriterReportsFlushError checks that http.ResponseController.Flush
// returns the flush error of the wrapped writer, which streaming handlers use
// to stop when the client goes away.
func TestServerWriterReportsFlushError(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	flushErr := errors.New("client gone")
	wrapped, invocation := wrapWithInterceptor(t, &flushErrorWriter{plainWriter: plainWriter{header: http.Header{}}, err: flushErr})
	assert.Equal(t, flushErr, http.NewResponseController(wrapped).Flush())
	require.NoError(t, (&ServerInterceptor{}).AfterInvoke(invocation))
	waitForSpans(t, 1)
}
