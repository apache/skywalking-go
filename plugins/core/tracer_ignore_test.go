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

package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

func TestIgnoreSuffix(t *testing.T) {
	ignoreSuffixStr := ".jpg,.jpeg,.js,.css,.png,.bmp,.gif,.ico,.mp3,.mp4,.html,.svg"
	ignoreSuffixList := strings.Split(ignoreSuffixStr, ",")
	assert.True(t, ignoreSuffix("GET:/favicon.ico", ignoreSuffixList))
}

func TestTraceIgnorePath(t *testing.T) {
	ignorePath := []string{"/health/*"}
	assert.False(t, traceIgnorePath("", ignorePath))
	assert.True(t, traceIgnorePath("/health/apps", ignorePath))
	assert.True(t, traceIgnorePath("/health/", ignorePath))
	assert.True(t, traceIgnorePath("/health/apps/", ignorePath))

	ignorePath = []string{"/health/**"}
	assert.True(t, traceIgnorePath("/health/apps/", ignorePath))

	ignorePath = []string{"/health/?"}
	assert.True(t, traceIgnorePath("/health/a", ignorePath))
	assert.False(t, traceIgnorePath("/health/ab", ignorePath))

	ignorePath = []string{"/health/*/"}
	assert.True(t, traceIgnorePath("/health/apps/", ignorePath))
	assert.False(t, traceIgnorePath("/health/", ignorePath))
	assert.False(t, traceIgnorePath("/health/apps/list", ignorePath))
	assert.False(t, traceIgnorePath("/health/test", ignorePath))

	ignorePath = []string{"/health/**"}
	assert.True(t, traceIgnorePath("/health/", ignorePath))
	assert.True(t, traceIgnorePath("/health/apps/test", ignorePath))
	assert.True(t, traceIgnorePath("/health/apps/test/", ignorePath))

	ignorePath = []string{"health/apps/?"}
	assert.False(t, traceIgnorePath("health/apps/list", ignorePath))
	assert.False(t, traceIgnorePath("health/apps/", ignorePath))
	assert.True(t, traceIgnorePath("health/apps/a", ignorePath))

	ignorePath = []string{"health/**/lists"}
	assert.True(t, traceIgnorePath("health/apps/lists", ignorePath))
	assert.True(t, traceIgnorePath("health/apps/test/lists", ignorePath))
	assert.False(t, traceIgnorePath("health/apps/test/", ignorePath))
	assert.False(t, traceIgnorePath("health/apps/test", ignorePath))

	ignorePath = []string{"health/**/test/**"}
	assert.True(t, traceIgnorePath("health/apps/test/list", ignorePath))
	assert.True(t, traceIgnorePath("health/apps/foo/test/list/bar", ignorePath))
	assert.True(t, traceIgnorePath("health/apps/foo/test/list/bar/", ignorePath))
	assert.True(t, traceIgnorePath("health/apps/test/list", ignorePath))
	assert.True(t, traceIgnorePath("health/test/list", ignorePath))

	ignorePath = []string{"/health/**/b/**/*.txt", "abc/*"}
	assert.True(t, traceIgnorePath("/health/a/aa/aaa/b/bb/bbb/xxxxxx.txt", ignorePath))
	assert.False(t, traceIgnorePath("/health/a/aa/aaa/b/bb/bbb/xxxxxx", ignorePath))
	assert.False(t, traceIgnorePath("abc/foo/bar", ignorePath))
	assert.True(t, traceIgnorePath("abc/foo", ignorePath))
}

func noHeader(string) (string, error) {
	return "", nil
}

// recordInjector returns an injector that records the headers it receives.
func recordInjector(headers map[string]string) tracing.Injector {
	return func(key, value string) error {
		headers[key] = value
		return nil
	}
}

// finishTrace ends a traced request on the current goroutine. The goroutine
// keeps its tracing context afterwards, as the goroutine of an HTTP/1
// keep-alive connection does between requests.
func finishTrace(t *testing.T) {
	t.Helper()
	s, err := tracing.CreateEntrySpan("GET:/first", noHeader)
	assert.NoError(t, err)
	assert.NotEqual(t, noopContextValue, s.TraceID(), "the first request should be traced")
	s.End()
	assert.NotNil(t, getTracingContext(), "the goroutine should keep the finished tracing context")
	assert.Nil(t, Tracing.ActiveSpan(), "no span should be active after the request")
}

func TestIgnoreRulesApplyAfterFinishedTrace(t *testing.T) {
	defer ResetTracingContext()
	Tracing.ignoreSuffix = []string{".html"}
	Tracing.traceIgnorePath = []string{"/health"}

	for _, operation := range []string{"GET:/ignored.html", "/health"} {
		finishTrace(t)
		s, err := tracing.CreateEntrySpan(operation, noHeader)
		assert.NoError(t, err)
		assert.Equalf(t, noopContextValue, s.TraceID(), "%s should be ignored", operation)
		s.End()
		assert.Nilf(t, Tracing.ActiveSpan(), "no span should be active after %s", operation)
	}
}

func TestDisconnectedReporterAppliesAfterFinishedTrace(t *testing.T) {
	defer ResetTracingContext()
	finishTrace(t)

	ReportConnectionStatus = reporter.ConnectionStatusDisconnect
	s, err := tracing.CreateEntrySpan("GET:/second", noHeader)
	assert.NoError(t, err)
	assert.Equal(t, noopContextValue, s.TraceID(), "nothing should be traced while the reporter is disconnected")
	s.End()
	assert.Nil(t, Tracing.ActiveSpan())
}

func TestSpansUnderIgnoredEntryAreIgnored(t *testing.T) {
	defer ResetTracingContext()
	Tracing.ignoreSuffix = []string{".html"}
	entry, err := tracing.CreateEntrySpan("GET:/ignored.html", noHeader)
	assert.NoError(t, err)
	assert.Equal(t, noopContextValue, entry.TraceID())

	// Two calls one after the other, so the first one ending must leave the
	// ignored entry active.
	for _, operation := range []string{"GET:/first-call", "GET:/second-call"} {
		headers := map[string]string{}
		exit, exitErr := tracing.CreateExitSpan(operation, "localhost:8080", recordInjector(headers))
		assert.NoError(t, exitErr)
		assert.Equalf(t, noopContextValue, exit.TraceID(), "%s under an ignored entry should be ignored", operation)
		assert.Emptyf(t, headers, "%s under an ignored entry should not propagate a context", operation)
		exit.End()
		_, isNoop := Tracing.ActiveSpan().(*NoopSpan)
		assert.Truef(t, isNoop, "the ignored entry should stay active after %s ends", operation)
	}
	entry.End()
	assert.Nil(t, Tracing.ActiveSpan(), "no span should be active after the ignored request")

	next, err := tracing.CreateEntrySpan("GET:/next", noHeader)
	assert.NoError(t, err)
	assert.NotEqual(t, noopContextValue, next.TraceID(), "the next request should be traced")
	next.End()
}

// The ignore rules apply when a span starts a tracing context, not to the
// spans inside a trace.
func TestIgnoreRulesDoNotApplyInsideTrace(t *testing.T) {
	defer ResetTracingContext()
	Tracing.ignoreSuffix = []string{".html"}
	entry, err := tracing.CreateEntrySpan("GET:/entry", noHeader)
	assert.NoError(t, err)

	headers := map[string]string{}
	exit, err := tracing.CreateExitSpan("GET:/page.html", "localhost:8080", recordInjector(headers))
	assert.NoError(t, err)
	assert.Equal(t, entry.TraceID(), exit.TraceID(), "a span inside a trace should be a child span")
	assert.NotEmpty(t, headers[Header], "the exit span should propagate the context")
	exit.End()

	if active := tracing.ActiveSpan(); assert.NotNil(t, active, "the entry span should stay active") {
		assert.Equal(t, entry.TraceID(), active.TraceID())
	}
	entry.End()
	waitReportedSpans(t, 2)
}

// One snapshot of an ignored request can be continued by several goroutines,
// such as the send and receive goroutines of a gRPC stream. The test GLS is
// shared, so each goroutine is simulated by switching its context.
func TestContinuedNoopSnapshotsAreIndependent(t *testing.T) {
	defer ResetTracingContext()
	Tracing.ignoreSuffix = []string{".html"}
	entry, err := tracing.CreateEntrySpan("GET:/ignored.html", noHeader)
	assert.NoError(t, err)
	requestCtx := getTracingContext()
	snapshot := tracing.CaptureContext()

	continueOnNewGoroutine := func() *TracingContext {
		SetGLS(nil)
		tracing.ContinueContext(snapshot)
		return getTracingContext()
	}
	first := continueOnNewGoroutine()
	second := continueOnNewGoroutine()

	SetGLS(first)
	firstSpan, err := tracing.CreateLocalSpan("first")
	assert.NoError(t, err)
	assert.Equal(t, noopContextValue, firstSpan.TraceID())
	SetGLS(second)
	secondSpan, err := tracing.CreateLocalSpan("second")
	assert.NoError(t, err)
	assert.Equal(t, noopContextValue, secondSpan.TraceID())

	SetGLS(first)
	firstSpan.End()
	assert.Nil(t, first.ActiveSpan(), "the first goroutine's span ended, so nothing should be active there")
	_, isNoop := second.ActiveSpan().(*NoopSpan)
	assert.True(t, isNoop, "the second goroutine's span is still open")
	SetGLS(second)
	secondSpan.End()
	assert.Nil(t, second.ActiveSpan())

	SetGLS(requestCtx)
	entry.End()
	assert.Nil(t, Tracing.ActiveSpan())
}
