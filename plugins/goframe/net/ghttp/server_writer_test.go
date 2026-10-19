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

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gf "github.com/gogf/gf/v2/net/ghttp"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newInterceptedServer serves handler on a real net/http server, behind the
// Server.ServeHTTP interceptor, the way the instrumented GoFrame server runs it.
func newInterceptedServer(t *testing.T, handler http.Handler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		interceptor := &GoFrameServerInterceptor{}
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
		handler.ServeHTTP(invocation.Args()[0].(http.ResponseWriter), r)
	}))
}

// newGoFrameServer starts a GoFrame server, so that its request handling is
// fully initialized, and returns it for use as a handler.
func newGoFrameServer(t *testing.T, pattern string, handler func(r *gf.Request)) *gf.Server {
	s := gf.GetServer(t.Name())
	s.SetDumpRouterMap(false)
	s.SetAddr("127.0.0.1:0")
	s.BindHandler(pattern, handler)
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Shutdown() })
	return s
}

func waitForSpans(t *testing.T, count int) {
	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < count {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d reported spans, got %d", count, len(core.GetReportedSpans()))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestGoFrameWebSocketUpgrade upgrades a GoFrame request to a WebSocket.
// GoFrame hijacks the connection with an unchecked w.(http.Hijacker), so the
// wrapped writer must keep http.Hijacker.
func TestGoFrameWebSocketUpgrade(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	s := newGoFrameServer(t, "/ws", func(r *gf.Request) {
		ws, err := r.WebSocket() //nolint:staticcheck // deprecated, but available in every supported version
		if err != nil {
			r.Response.WriteStatus(http.StatusInternalServerError, err.Error())
			return
		}
		_ = ws.Close()
	})
	server := newInterceptedServer(t, s)
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/ws", http.NoBody)
	require.NoError(t, err)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	resp, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode, "upgrade failed: %s", body)
	waitForSpans(t, 1)
}

// TestGoFrameFlushReachesClient is the server-sent events case: the first event
// flushed by r.Response.Flush must reach the client while the handler runs.
func TestGoFrameFlushReachesClient(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	received := make(chan struct{})
	s := newGoFrameServer(t, "/events", func(r *gf.Request) {
		r.Response.Header().Set("Content-Type", "text/event-stream")
		r.Response.Write("data: ping\n\n")
		r.Response.Flush()
		select {
		case <-received:
			r.Response.Write("data: done\n\n")
		case <-time.After(5 * time.Second):
			r.Response.Write("data: timeout\n\n")
		}
	})
	server := newInterceptedServer(t, s)
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

// TestServerWriterKeepsOptionalInterfaces checks that a handler sees the same
// optional features through the wrapped writer as through the server's own
// writer: http.Flusher and http.Hijacker by type assertion, and everything
// http.ResponseController reaches through Unwrap.
func TestServerWriterKeepsOptionalInterfaces(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	server := newInterceptedServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, isFlusher := w.(http.Flusher)
		_, isHijacker := w.(http.Hijacker)
		controller := http.NewResponseController(w)
		deadlineErr := controller.SetWriteDeadline(time.Now().Add(time.Minute))
		duplexErr := controller.EnableFullDuplex()
		flushErr := controller.Flush()
		_, _ = fmt.Fprintf(w, "flusher=%v hijacker=%v deadline=%v duplex=%v flush=%v",
			isFlusher, isHijacker, deadlineErr, duplexErr, flushErr)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/features")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, "flusher=true hijacker=true deadline=<nil> duplex=<nil> flush=<nil>", string(body))
	waitForSpans(t, 1)
}

// plainWriter implements only http.ResponseWriter.
type plainWriter struct {
	header http.Header
}

func (p *plainWriter) Header() http.Header         { return p.header }
func (p *plainWriter) Write(b []byte) (int, error) { return len(b), nil }
func (p *plainWriter) WriteHeader(int)             {}

func wrapWithInterceptor(t *testing.T, writer http.ResponseWriter) (http.ResponseWriter, operator.Invocation) {
	request, err := http.NewRequest(http.MethodGet, "http://localhost/", http.NoBody)
	require.NoError(t, err)
	invocation := operator.NewInvocation(nil, writer, request)
	require.NoError(t, (&GoFrameServerInterceptor{}).BeforeInvoke(invocation))
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
	require.NoError(t, (&GoFrameServerInterceptor{}).AfterInvoke(invocation))

	wrapped, invocation = wrapWithInterceptor(t, &plainWriter{header: http.Header{}})
	_, isHijacker = wrapped.(http.Hijacker)
	_, isFlusher = wrapped.(http.Flusher)
	assert.False(t, isHijacker, "the wrapped writer cannot hijack")
	assert.False(t, isFlusher, "the wrapped writer cannot flush")
	assert.True(t, errors.Is(http.NewResponseController(wrapped).Flush(), http.ErrNotSupported))
	require.NoError(t, (&GoFrameServerInterceptor{}).AfterInvoke(invocation))
	waitForSpans(t, 2)
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
	require.NoError(t, (&GoFrameServerInterceptor{}).AfterInvoke(invocation))
	waitForSpans(t, 1)
}
