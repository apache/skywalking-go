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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	tracedPage  = "GET:/page"
	ignoredPage = "GET:/page.html"
)

type failMode int

const (
	failNone failMode = iota
	failError
	failPanic
)

// spanInterceptor creates an entry or exit span in BeforeInvoke and ends it in
// AfterInvoke, with the defects found in plugins.
type spanInterceptor struct {
	operation string
	exit      bool
	// fail is how BeforeInvoke fails after it creates the span and before SetContext
	fail failMode
	// skipEnd makes AfterInvoke return without ending the span
	skipEnd bool
	// panicInAfter makes AfterInvoke panic before ending the span
	panicInAfter bool
	// skipCall defines the return values, so the method and AfterInvoke don't run
	skipCall bool
	// failBeforeCreate makes BeforeInvoke return an error before it creates the span
	failBeforeCreate bool
	// async makes AfterInvoke end the span in async mode; asyncSpan keeps it
	async     bool
	asyncSpan tracing.Span
}

func (h *spanInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	if h.failBeforeCreate {
		return errors.New("the plugin failed before creating the span")
	}
	var s tracing.Span
	var err error
	if h.exit {
		s, err = tracing.CreateExitSpan(h.operation, "localhost:8080", func(string, string) error { return nil })
	} else {
		s, err = tracing.CreateEntrySpan(h.operation, noHeader)
	}
	if err != nil {
		return err
	}
	switch h.fail {
	case failError:
		return errors.New("the plugin failed after creating the span")
	case failPanic:
		panic("the plugin panicked after creating the span")
	}
	invocation.SetContext(s)
	if h.skipCall {
		invocation.DefineReturnValues()
	}
	return nil
}

func (h *spanInterceptor) AfterInvoke(invocation operator.Invocation, _ ...interface{}) error {
	s, ok := invocation.GetContext().(tracing.Span)
	if !ok {
		return nil
	}
	if h.panicInAfter {
		panic("the plugin panicked before ending the span")
	}
	if h.async {
		s.PrepareAsync()
		h.asyncSpan = s
	}
	if !h.skipEnd {
		s.End()
	}
	return nil
}

// intercept runs the interceptor around call on the current goroutine, as the
// generated code does.
func intercept(h *spanInterceptor, call func()) {
	operator.InvokeIntercepted(h, operator.NewInvocation(nil), func() []interface{} {
		if call != nil {
			call()
		}
		return nil
	})
}

func activeTraceID() string {
	if s := tracing.ActiveSpan(); s != nil {
		return s.TraceID()
	}
	return ""
}

// activeTracingSpan returns the active span of the current goroutine.
func activeTracingSpan() TracingSpan {
	s, _ := Tracing.ActiveSpan().(TracingSpan)
	return s
}

// assertNewTrace checks that the span starts a new trace: it is traced, and it
// is a root span, not the leaked span or one of its children. Trace IDs cannot
// tell traces apart in these tests: two contexts created in the same
// millisecond on the shared test goroutine get the same IDs.
func assertNewTrace(t *testing.T, span, leaked TracingSpan, msgAndArgs ...interface{}) {
	t.Helper()
	if !assert.NotNil(t, span, msgAndArgs...) {
		return
	}
	_, isNoop := span.(*NoopSpan)
	assert.False(t, isNoop, msgAndArgs...)
	assert.Nil(t, span.ParentSpan(), msgAndArgs...)
	if leaked != nil {
		assert.NotSame(t, leaked, span, msgAndArgs...)
	}
}

// assertNextRequestTraced checks that the goroutine is clean: a new request
// starts its own trace.
func assertNextRequestTraced(t *testing.T) {
	t.Helper()
	assert.Nil(t, Tracing.ActiveSpan(), "no span should be active between requests")
	var next TracingSpan
	intercept(&spanInterceptor{operation: "GET:/next"}, func() { next = activeTracingSpan() })
	assertNewTrace(t, next, nil, "the next request should start a new trace")
	assert.Nil(t, Tracing.ActiveSpan())
}

func TestFailedBeforeInvokeEndsItsSpan(t *testing.T) {
	for _, fail := range []failMode{failError, failPanic} {
		for _, ignored := range []bool{true, false} {
			ResetTracingContext()
			Tracing.ignoreSuffix = []string{".html"}
			operation := tracedPage
			if ignored {
				operation = ignoredPage
			}
			intercept(&spanInterceptor{operation: operation, fail: fail}, nil)
			assert.Nilf(t, Tracing.ActiveSpan(), "fail=%d ignored=%v: the span of the failed hook should be ended", fail, ignored)
			if !ignored {
				waitReportedSpans(t, 1)
			}
			assertNextRequestTraced(t)
		}
	}
	ResetTracingContext()
}

func TestFailedNestedBeforeInvokeKeepsTheOuterSpan(t *testing.T) {
	cases := []struct {
		name  string
		outer string
		inner *spanInterceptor
	}{
		{"exit under entry", tracedPage, &spanInterceptor{operation: "GET:/downstream", exit: true, fail: failError}},
		{"reused entry", tracedPage, &spanInterceptor{operation: "GET:/page/inner", fail: failPanic}},
		{"noop under ignored entry", ignoredPage, &spanInterceptor{operation: "GET:/downstream", exit: true, fail: failError}},
	}
	for _, c := range cases {
		ResetTracingContext()
		Tracing.ignoreSuffix = []string{".html"}
		var outer, afterInner TracingSpan
		var outerDepth, afterInnerDepth int
		intercept(&spanInterceptor{operation: c.outer}, func() {
			outer = activeTracingSpan()
			outerDepth = spanDepth(outer)
			intercept(c.inner, nil)
			afterInner = activeTracingSpan()
			afterInnerDepth = spanDepth(outer)
		})
		assert.Samef(t, outer, afterInner, "%s: the outer span should stay active", c.name)
		assert.Equalf(t, outerDepth, afterInnerDepth, "%s: the nesting of the failed hook should be undone", c.name)
		assert.Nilf(t, Tracing.ActiveSpan(), "%s: no span should be active after the request", c.name)
		if _, isNoop := outer.(*NoopSpan); !isNoop {
			// the outer span is reported: no owner is left on it
			waitReportedSpans(t, 1)
		}
		assertNextRequestTraced(t)
	}
	ResetTracingContext()
}

func TestSpanLeftByFinishedInterceptorIsEnded(t *testing.T) {
	leaks := []struct {
		name string
		h    *spanInterceptor
	}{
		{"after-hook without End", &spanInterceptor{skipEnd: true}},
		{"after-hook panics", &spanInterceptor{panicInAfter: true}},
		{"skipped call", &spanInterceptor{skipCall: true}},
	}
	for _, leak := range leaks {
		for _, ignored := range []bool{true, false} {
			ResetTracingContext()
			Tracing.ignoreSuffix = []string{".html"}
			leak.h.operation = tracedPage
			if ignored {
				leak.h.operation = ignoredPage
			}
			var leaked TracingSpan
			intercept(leak.h, func() { leaked = activeTracingSpan() })
			assert.NotNilf(t, Tracing.ActiveSpan(), "%s: the span is left active", leak.name)

			var next TracingSpan
			intercept(&spanInterceptor{operation: "GET:/next"}, func() { next = activeTracingSpan() })
			assertNewTrace(t, next, leaked, "%s ignored=%v: the next request should start a new trace", leak.name, ignored)
			assert.Nilf(t, Tracing.ActiveSpan(), "%s ignored=%v", leak.name, ignored)
			if !ignored {
				// the leaked span is reported too
				waitReportedSpans(t, 2)
			}
		}
	}
	ResetTracingContext()
}

// A span stays active while the interceptor that created it is running, even
// after nested interceptors finished.
func TestSpanOfRunningInterceptorIsKept(t *testing.T) {
	defer ResetTracingContext()
	var entryTraceID, exitTraceID string
	intercept(&spanInterceptor{operation: tracedPage}, func() {
		entryTraceID = activeTraceID()
		// a nested interceptor that reuses the entry span, such as gin inside
		// the net/http server, leaves it active when it ends
		intercept(&spanInterceptor{operation: "GET:/page/inner"}, nil)
		intercept(&spanInterceptor{operation: "GET:/first", exit: true}, func() {
			intercept(&spanInterceptor{operation: "GET:/nested", exit: true}, nil)
		})
		intercept(&spanInterceptor{operation: "GET:/second", exit: true}, func() { exitTraceID = activeTraceID() })
	})
	assert.Equal(t, entryTraceID, exitTraceID, "the exit spans should join the running request")
	assert.Nil(t, Tracing.ActiveSpan())
	waitReportedSpans(t, 3)
}

// A span that no interceptor stored, such as a toolkit span, is never ended by
// the agent.
func TestSpanWithoutOwnerIsKept(t *testing.T) {
	defer ResetTracingContext()
	local, err := tracing.CreateLocalSpan("toolkit")
	assert.NoError(t, err)
	intercept(&spanInterceptor{operation: "GET:/downstream", exit: true, fail: failError}, nil)
	intercept(&spanInterceptor{operation: "GET:/downstream", exit: true, skipEnd: true}, nil)
	var traceID string
	intercept(&spanInterceptor{operation: "GET:/downstream", exit: true}, func() { traceID = activeTraceID() })
	assert.Equal(t, local.TraceID(), traceID, "spans should still join the toolkit span")
	assert.Equal(t, local.TraceID(), activeTraceID(), "the toolkit span should stay active")
	local.End()
	assert.Nil(t, Tracing.ActiveSpan())
}

// The spans created after a nested plugin ends a reused entry span, while the
// outer plugin still serves the request, belong to the request.
func TestReusedEntrySpanStaysActiveAfterNestedEnd(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	var outerID, afterInnerID, exitID string
	intercept(&spanInterceptor{operation: "GET:/raw"}, func() {
		outerID = activeTraceID()
		intercept(&spanInterceptor{operation: "GET:/renamed"}, nil)
		afterInnerID = activeTraceID()
		intercept(&spanInterceptor{operation: "GET:/downstream", exit: true}, func() { exitID = activeTraceID() })
	})
	assert.NotEmpty(t, outerID)
	assert.Equal(t, outerID, afterInnerID, "the reused entry span should stay active after the nested End")
	assert.Equal(t, outerID, exitID, "the exit span should join the request")
	assert.Nil(t, Tracing.ActiveSpan())
	spans := waitReportedSpans(t, 2)
	assert.Equal(t, 2, len(spans))
	assert.NotNil(t, findReportedSpan(spans, "GET:/renamed"), "the reused span keeps the nested rename")
}

// The last End of a reused entry span makes its parent active, whichever
// owner ends last.
func TestReusedEntrySpanOuterEndsFirst(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	outer, err := tracing.CreateEntrySpan("GET:/raw", noHeader)
	assert.NoError(t, err)
	inner, err := tracing.CreateEntrySpan("GET:/renamed", noHeader)
	assert.NoError(t, err)
	// the inner owner is an interceptor
	operator.NewInvocation(nil).SetContext(inner)
	outer.End()
	assert.Equal(t, outer.TraceID(), activeTraceID(), "the span stays active until its last owner ends")
	inner.End()
	assert.Nil(t, Tracing.ActiveSpan())
	assert.Equal(t, 1, len(waitReportedSpans(t, 1)))
}

// gorm creates an exit span that the sql driver reuses. After the driver ends,
// the exit span is no longer active, so a gorm hook query gets its own span.
func TestReusedExitSpanIsPoppedAfterNestedEnd(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	entry, err := tracing.CreateEntrySpan("GET:/users", noHeader)
	assert.NoError(t, err)
	gormSpan, err := tracing.CreateExitSpan("users/create", "db:3306", func(string, string) error { return nil })
	assert.NoError(t, err)
	intercept(&spanInterceptor{operation: "MySQL/Exec", exit: true}, nil)
	active := activeTracingSpan()
	if assert.NotNil(t, active) {
		assert.True(t, active.IsEntry(), "the entry span should be active after the driver ended the reused exit span")
	}
	intercept(&spanInterceptor{operation: "users/hook", exit: true}, nil)
	gormSpan.Tag("db.statement", "INSERT INTO users")
	gormSpan.End()
	entry.End()
	spans := waitReportedSpans(t, 3)
	assert.Equal(t, 3, len(spans), "the hook query should have its own span")
	if s := findReportedSpan(spans, "users/create"); assert.NotNil(t, s) {
		v, ok := reportedTagValue(s, "db.statement")
		assert.True(t, ok)
		assert.Equal(t, "INSERT INTO users", v)
	}
}

// A span whose creator is not an interceptor, such as a toolkit or gorm
// callback span, is not owned by a nested interceptor that reuses it.
func TestNestedInterceptorDoesNotOwnUnboundSpan(t *testing.T) {
	for _, ignored := range []bool{false, true} {
		ResetTracingContext()
		Tracing.ignoreSuffix = []string{".html"}
		operation := "GET:/toolkit"
		if ignored {
			operation = "GET:/toolkit.html"
		}
		root, err := tracing.CreateEntrySpan(operation, noHeader)
		assert.NoError(t, err)
		intercept(&spanInterceptor{operation: "GET:/nested"}, nil)
		intercept(&spanInterceptor{operation: "GET:/downstream", exit: true}, nil)
		var traceID string
		intercept(&spanInterceptor{operation: "GET:/downstream", exit: true}, func() { traceID = activeTraceID() })
		assert.Equalf(t, root.TraceID(), traceID, "ignored=%v: the spans should still join the root span", ignored)
		assert.Equalf(t, root.TraceID(), activeTraceID(), "ignored=%v: the root span should stay active", ignored)
		root.End()
		assert.Nilf(t, Tracing.ActiveSpan(), "ignored=%v", ignored)
	}
	ResetTracingContext()
}

// endActiveThenFail ends the active span and then fails, so the active span
// before the hook is no longer an ancestor of the active span.
type endActiveThenFail struct{}

func (endActiveThenFail) BeforeInvoke(operator.Invocation) error {
	if s := tracing.ActiveSpan(); s != nil {
		s.End()
	}
	return errors.New("failed after ending the active span")
}

func (endActiveThenFail) AfterInvoke(operator.Invocation, ...interface{}) error {
	return nil
}

func TestRollbackStopsWhenMarkedSpanEnded(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	entry, err := tracing.CreateEntrySpan(tracedPage, noHeader)
	assert.NoError(t, err)
	local, err := tracing.CreateLocalSpan("local")
	assert.NoError(t, err)
	operator.InvokeIntercepted(endActiveThenFail{}, operator.NewInvocation(nil), func() []interface{} { return nil })
	active := activeTracingSpan()
	if assert.NotNil(t, active, "the entry span should not be rolled back") {
		assert.True(t, active.IsEntry())
	}
	_ = local
	entry.End()
	assert.Nil(t, Tracing.ActiveSpan())
}

func TestFailedBeforeInvokeBeforeCreatingSpanChangesNothing(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	var traceID, afterID string
	intercept(&spanInterceptor{operation: tracedPage}, func() {
		traceID = activeTraceID()
		intercept(&spanInterceptor{operation: "GET:/downstream", exit: true, failBeforeCreate: true}, nil)
		afterID = activeTraceID()
	})
	assert.Equal(t, traceID, afterID)
	assert.Nil(t, Tracing.ActiveSpan())
	assert.Equal(t, 1, len(waitReportedSpans(t, 1)))
}

// The noop spans of a disconnected reporter and of the sampler are reclaimed
// and rolled back like the noop span of an ignored request.
func TestNoopSpansOfOtherPathsAreReclaimed(t *testing.T) {
	setups := map[string]func(){
		"disconnected reporter": func() { ReportConnectionStatus = reporter.ConnectionStatusDisconnect },
		"sampled out":           func() { Tracing.Sampler = NewConstSampler(false) },
	}
	for name, setup := range setups {
		for _, h := range []*spanInterceptor{
			{operation: tracedPage, skipEnd: true},
			{operation: tracedPage, fail: failError},
			{operation: tracedPage, fail: failPanic},
		} {
			ResetTracingContext()
			setup()
			intercept(h, nil)
			ReportConnectionStatus = reporter.ConnectionStatusConnected
			Tracing.Sampler = NewConstSampler(true)
			var traceID string
			intercept(&spanInterceptor{operation: "GET:/next"}, func() { traceID = activeTraceID() })
			assert.NotEqualf(t, noopContextValue, traceID, "%s %+v: the next request should be traced", name, *h)
			assert.Nilf(t, Tracing.ActiveSpan(), "%s", name)
		}
	}
	ResetTracingContext()
}

func TestReclaimedSpansAreCountedAsLeaked(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	Tracing.ignoreSuffix = []string{".html"}
	so11y := GetSo11y(Tracing)
	leaked, leakedIgnored := so11y.leakedContextCounter.Get(), so11y.leakedIgnoreContextCounter.Get()

	intercept(&spanInterceptor{operation: tracedPage, skipEnd: true}, nil)
	intercept(&spanInterceptor{operation: "GET:/next"}, nil)
	assert.Equal(t, leaked+1, so11y.leakedContextCounter.Get())

	intercept(&spanInterceptor{operation: ignoredPage, skipEnd: true}, nil)
	intercept(&spanInterceptor{operation: "GET:/next"}, nil)
	assert.Equal(t, leakedIgnored+1, so11y.leakedIgnoreContextCounter.Get())

	// a rolled back span is counted too
	intercept(&spanInterceptor{operation: tracedPage, fail: failError}, nil)
	assert.Equal(t, leaked+2, so11y.leakedContextCounter.Get())
}

// A goroutine that continues a captured context keeps the snapshot span: only
// the leaked spans created on it are ended.
func TestContinuedContextKeepsSnapshotSpan(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	entry, err := tracing.CreateEntrySpan(tracedPage, noHeader)
	assert.NoError(t, err)
	requestCtx := getTracingContext()
	snapshot := tracing.CaptureContext()

	SetGLS(nil)
	tracing.ContinueContext(snapshot)
	intercept(&spanInterceptor{operation: "GET:/leaked", exit: true, skipEnd: true}, nil)
	var traceID string
	intercept(&spanInterceptor{operation: "GET:/next", exit: true}, func() { traceID = activeTraceID() })
	assert.Equal(t, entry.TraceID(), traceID, "the continued goroutine should stay in the trace")
	_, isSnapshot := Tracing.ActiveSpan().(*SnapshotSpan)
	assert.True(t, isSnapshot, "the snapshot span should stay active")

	SetGLS(requestCtx)
	entry.End()
	assert.Nil(t, Tracing.ActiveSpan())
}

// A goroutine that continues an ignored request owns its first noop span, so a
// leak there is reclaimed instead of ignoring the goroutine forever.
func TestContinuedNoopSnapshotLeakIsReclaimed(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	Tracing.ignoreSuffix = []string{".html"}
	entry, err := tracing.CreateEntrySpan("GET:/ignored.html", noHeader)
	assert.NoError(t, err)
	requestCtx := getTracingContext()
	snapshot := tracing.CaptureContext()

	SetGLS(nil)
	tracing.ContinueContext(snapshot)
	intercept(&spanInterceptor{operation: "GET:/leaked", exit: true, skipEnd: true}, nil)
	var traceID string
	intercept(&spanInterceptor{operation: "GET:/next"}, func() { traceID = activeTraceID() })
	assert.NotEqual(t, noopContextValue, traceID, "the next request on the continued goroutine should be traced")
	assert.Nil(t, Tracing.ActiveSpan())

	SetGLS(requestCtx)
	entry.End()
	assert.Nil(t, Tracing.ActiveSpan())
}

func TestAsyncSpanIsNotReclaimed(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	async := &spanInterceptor{operation: "GET:/stream", exit: true, async: true}
	var traceID, nextID string
	intercept(&spanInterceptor{operation: tracedPage}, func() {
		traceID = activeTraceID()
		intercept(async, nil)
		intercept(&spanInterceptor{operation: "GET:/next", exit: true}, func() { nextID = activeTraceID() })
	})
	assert.Equal(t, traceID, nextID)
	if assert.NotNil(t, async.asyncSpan) {
		async.asyncSpan.AsyncFinish()
	}
	assert.Equal(t, 3, len(waitReportedSpans(t, 3)))
}

// continueThenFail continues a captured context, optionally creates a span on
// it, and then fails, as a stream hook that fails after ContinueContext.
type continueThenFail struct {
	snapshot   tracing.ContextSnapshot
	createSpan bool
	panics     bool
}

func (h *continueThenFail) BeforeInvoke(operator.Invocation) error {
	tracing.ContinueContext(h.snapshot)
	if h.createSpan {
		if _, err := tracing.CreateLocalSpan("on-snapshot"); err != nil {
			return err
		}
	}
	if h.panics {
		panic("the hook panicked after continuing the context")
	}
	return errors.New("the hook failed after continuing the context")
}

func (h *continueThenFail) AfterInvoke(operator.Invocation, ...interface{}) error {
	return nil
}

// A failed hook keeps the context it continued: a snapshot span cannot be
// ended on this goroutine, and its End panics.
func TestRollbackKeepsContinuedSnapshot(t *testing.T) {
	for _, ignored := range []bool{false, true} {
		for _, createSpan := range []bool{false, true} {
			for _, panics := range []bool{false, true} {
				ResetTracingContext()
				Tracing.ignoreSuffix = []string{".html"}
				operation := tracedPage
				if ignored {
					operation = ignoredPage
				}
				entry, err := tracing.CreateEntrySpan(operation, noHeader)
				assert.NoError(t, err)
				requestCtx := getTracingContext()
				snapshot := tracing.CaptureContext()
				so11y := GetSo11y(Tracing)
				leaked, leakedIgnored := so11y.leakedContextCounter.Get(), so11y.leakedIgnoreContextCounter.Get()

				SetGLS(nil)
				h := &continueThenFail{snapshot: snapshot, createSpan: createSpan, panics: panics}
				name := fmt.Sprintf("ignored=%v span=%v panic=%v", ignored, createSpan, panics)
				assert.NotPanicsf(t, func() {
					operator.InvokeIntercepted(h, operator.NewInvocation(nil), func() []interface{} { return nil })
				}, "%s", name)
				active := activeTracingSpan()
				assert.Truef(t, active != nil && isContinuedSnapshot(active), "%s: the continued context should stay active, got %T", name, active)
				if !createSpan {
					assert.Equalf(t, leaked, so11y.leakedContextCounter.Get(), "%s", name)
					assert.Equalf(t, leakedIgnored, so11y.leakedIgnoreContextCounter.Get(), "%s", name)
				}

				SetGLS(requestCtx)
				entry.End()
			}
		}
	}
	ResetTracingContext()
}

// A pull consumer inside a request creates and ends an entry span per message
// without storing it. The first message reuses the request entry span, as
// before, and the next ones get their own span linked to the producer.
func TestPullConsumerInsideRequestKeepsUpstreamLink(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	headers := map[string]string{}
	producer, err := tracing.CreateEntrySpan("producer", noHeader)
	assert.NoError(t, err)
	send, err := tracing.CreateExitSpan("send", "mq:6650", func(k, v string) error {
		headers[k] = v
		return nil
	})
	assert.NoError(t, err)
	send.End()
	producer.End()
	waitReportedSpans(t, 2)
	SetGLS(nil)

	assert.NotEmpty(t, headers[Header])
	Tracing.Reporter = &StoreReporter{}

	var request TracingSpan
	var consumed []TracingSpan
	intercept(&spanInterceptor{operation: "GET:/http"}, func() {
		request = activeTracingSpan()
		for i := 0; i < 2; i++ {
			_, err := tracing.CreateEntrySpan("consume", func(k string) (string, error) { return headers[k], nil })
			assert.NoError(t, err)
			consumed = append(consumed, activeTracingSpan())
			tracing.ActiveSpan().End()
		}
	})
	if assert.Len(t, consumed, 2) {
		assert.Same(t, request, consumed[0], "the first message reuses the request entry span")
		assert.NotSame(t, request, consumed[1], "the next message should get its own span")
	}
	assert.Nil(t, Tracing.ActiveSpan())
	// the request span, renamed by the reuse, and the second message's span
	// with the producer reference
	spans := waitReportedSpans(t, 2)
	var linked int
	for _, s := range spans {
		if s.OperationName() == "consume" && len(s.Refs()) > 0 {
			linked++
		}
	}
	assert.Equal(t, 2, len(spans))
	assert.Equal(t, 1, linked, "the second message should be linked to the producer")
}

// A failed hook after an orphan rolls back its own span: the mark is taken
// after the orphan is ended.
func TestFailedHookAfterOrphanIsRolledBack(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	so11y := GetSo11y(Tracing)
	leaked := so11y.leakedContextCounter.Get()
	intercept(&spanInterceptor{operation: tracedPage, skipEnd: true}, nil)
	for _, fail := range []failMode{failError, failPanic} {
		intercept(&spanInterceptor{operation: "GET:/second", fail: fail}, nil)
		assert.Nilf(t, Tracing.ActiveSpan(), "fail=%d: the span of the failed hook should be ended", fail)
	}
	assert.Equal(t, leaked+3, so11y.leakedContextCounter.Get(), "the orphan and both failed spans are counted")
	assertNextRequestTraced(t)
}

// An End after AsyncFinish ended a reused entry span still makes its parent
// active, so the span is not left active and counted as leaked.
func TestReusedEntrySpanFinishedAsyncIsPopped(t *testing.T) {
	defer ResetTracingContext()
	ResetTracingContext()
	so11y := GetSo11y(Tracing)
	leaked := so11y.leakedContextCounter.Get()
	async := &spanInterceptor{operation: "GET:/async", async: true}
	intercept(&spanInterceptor{operation: "GET:/raw"}, func() {
		intercept(async, nil)
		if assert.NotNil(t, async.asyncSpan) {
			async.asyncSpan.AsyncFinish()
		}
	})
	assert.Nil(t, Tracing.ActiveSpan(), "the outer End should pop the span")
	assertNextRequestTraced(t)
	assert.Equal(t, leaked, so11y.leakedContextCounter.Get(), "nothing leaked")
}

func BenchmarkMarkActiveSpan(b *testing.B) {
	defer ResetTracingContext()
	marker := operator.GetOperator().Tracing().(interface {
		ActiveSpanMark() (interface{}, int)
	})
	s, _ := tracing.CreateEntrySpan("GET:/bench", noHeader)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = marker.ActiveSpanMark()
	}
	b.StopTimer()
	s.End()
}

func BenchmarkMarkActiveSpanNoContext(b *testing.B) {
	defer ResetTracingContext()
	SetGLS(nil)
	marker := operator.GetOperator().Tracing().(interface {
		ActiveSpanMark() (interface{}, int)
	})
	for i := 0; i < b.N; i++ {
		_, _ = marker.ActiveSpanMark()
	}
}
