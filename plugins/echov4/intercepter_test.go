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

package echov4

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingLogger keeps the agent log lines written by the middleware.
type recordingLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *recordingLogger) record(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *recordingLogger) WithField(string, interface{}) interface{} { return l }
func (l *recordingLogger) Info(args ...interface{})                  { l.record("%s", fmt.Sprint(args...)) }
func (l *recordingLogger) Infof(format string, args ...interface{})  { l.record(format, args...) }
func (l *recordingLogger) Warn(args ...interface{})                  { l.record("%s", fmt.Sprint(args...)) }
func (l *recordingLogger) Warnf(format string, args ...interface{})  { l.record(format, args...) }
func (l *recordingLogger) Error(args ...interface{})                 { l.record("%s", fmt.Sprint(args...)) }
func (l *recordingLogger) Errorf(format string, args ...interface{}) { l.record(format, args...) }

func (l *recordingLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

func resetTracing(t *testing.T) *recordingLogger {
	core.ResetTracingContext()
	logger := &recordingLogger{}
	core.Tracing.Log = &core.LogWrapper{Logger: logger}
	t.Cleanup(core.ResetTracingContext)
	return logger
}

// newEcho returns an Echo instance with the tracing middleware, as the
// instrumented echo.New installs it.
func newEcho() *echo.Echo {
	e := echo.New()
	e.Use(middleware())
	e.GET("/ok", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	e.GET("/panic", func(c echo.Context) error {
		panic("boom")
	})
	return e
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

func tagValue(span reporter.ReportedSpan, key string) (string, bool) {
	for _, tag := range span.Tags() {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

func TestRequestIsTraced(t *testing.T) {
	resetTracing(t)
	recorder := httptest.NewRecorder()
	newEcho().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", http.NoBody))

	assert.Equal(t, http.StatusOK, recorder.Code)
	spans := waitForSpans(t, 1)
	assert.Equal(t, "GET:/ok", spans[0].OperationName())
	code, _ := tagValue(spans[0], tracing.TagStatusCode)
	assert.Equal(t, "200", code)
	assert.False(t, spans[0].IsError())
}

// TestMalformedSW8HeaderIsServed sends a propagation header that cannot be
// decoded. The request must still reach the handler, untraced, and the decode
// error must be logged.
func TestMalformedSW8HeaderIsServed(t *testing.T) {
	logger := resetTracing(t)
	request := httptest.NewRequest(http.MethodGet, "/ok", http.NoBody)
	request.Header.Set("sw8", "x")
	recorder := httptest.NewRecorder()
	newEcho().ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
	assert.Contains(t, logger.String(), "GET:/ok", "the decode error must be logged")
	assert.Nil(t, tracing.ActiveSpan(), "no span may be left active")
	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, core.GetReportedSpans(), "the request is served untraced")
}

// TestPanicEndsSpan lets a handler panic with no recovery inside the Echo
// middleware chain. The span must be ended and reported as an error, and the
// panic must propagate unchanged.
func TestPanicEndsSpan(t *testing.T) {
	resetTracing(t)
	e := newEcho()
	recovered := func() (r interface{}) {
		defer func() { r = recover() }()
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", http.NoBody))
		return nil
	}()

	assert.Equal(t, "boom", recovered, "the panic must propagate unchanged")
	assert.Nil(t, tracing.ActiveSpan(), "the span must not stay active for the next request")
	spans := waitForSpans(t, 1)
	assert.Equal(t, "GET:/panic", spans[0].OperationName())
	assert.True(t, spans[0].IsError())
	_, hasStatus := tagValue(spans[0], tracing.TagStatusCode)
	assert.False(t, hasStatus, "the response status is not known yet when the handler panics")
	require.Len(t, spans[0].Logs(), 1)
	assert.Equal(t, abnormalEndMessage, spans[0].Logs()[0].Data[0].Key)
}

// TestNilPanicIsNotSwallowed panics with nil under GODEBUG=panicnil=1, where
// recover returns nil: the middleware must not stop the panic.
func TestNilPanicIsNotSwallowed(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	resetTracing(t)
	e := newEcho()
	e.GET("/nilpanic", func(c echo.Context) error {
		panic(nil) //nolint:govet // the nil panic is the case under test
	})
	panicked := true
	func() {
		defer func() { _ = recover() }()
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nilpanic", http.NoBody))
		panicked = false
	}()

	assert.True(t, panicked, "the nil panic must propagate")
	assert.Nil(t, tracing.ActiveSpan(), "the span must not stay active for the next request")
	spans := waitForSpans(t, 1)
	assert.True(t, spans[0].IsError())
}

// TestGoexitEndsSpan calls runtime.Goexit in a handler: the span must still end.
func TestGoexitEndsSpan(t *testing.T) {
	resetTracing(t)
	e := newEcho()
	e.GET("/goexit", func(c echo.Context) error {
		runtime.Goexit()
		return nil
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		core.SetAsNewGoroutine()
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/goexit", http.NoBody))
	}()
	<-done

	spans := waitForSpans(t, 1)
	assert.Equal(t, "GET:/goexit", spans[0].OperationName())
	assert.True(t, spans[0].IsError())
}

// TestPanicRecoveredByPreMiddleware recovers the panic in a Pre middleware,
// which runs outside the tracing middleware. The next request served by the
// same goroutine must get its own span.
func TestPanicRecoveredByPreMiddleware(t *testing.T) {
	resetTracing(t)
	e := newEcho()
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("recovered: %v", r)
				}
			}()
			return next(c)
		}
	})

	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", http.NoBody))
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	recorder = httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", http.NoBody))
	assert.Equal(t, http.StatusOK, recorder.Code)

	spans := waitForSpans(t, 2)
	isError := map[string]bool{}
	for _, span := range spans {
		isError[span.OperationName()] = span.IsError()
	}
	assert.Equal(t, map[string]bool{"GET:/panic": true, "GET:/ok": false}, isError)
}
