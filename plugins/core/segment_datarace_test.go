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

//go:build race

// This file holds data-race regression tests for apache/skywalking#13885. They are
// only meaningful under the race detector, so they are built solely with the `race`
// tag and are run by `make test-race` (which selects them via `-run '^TestRace'`).

package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/apache/skywalking-go/plugins/core/reporter"
)

// buildReportedSpan constructs a real *SegmentSpanImpl, the exact type the agent
// hands to Transform.TransformSegmentObject when a finished segment is reported.
func buildReportedSpan() *SegmentSpanImpl {
	return &SegmentSpanImpl{
		DefaultSpan: DefaultSpan{
			StartTime:     time.Now(),
			EndTime:       time.Now(),
			OperationName: "users/SELECT",
			Peer:          "127.0.0.1:5432",
			SpanType:      SpanTypeExit,
			opLock:        &sync.Mutex{},
		},
		SegmentContext: SegmentContext{
			TraceID:            "trace-id",
			SegmentID:          "segment-id",
			SpanID:             0,
			ParentSpanID:       -1,
			CorrelationContext: newCorrelationContext(),
		},
	}
}

// TestRaceSegmentDecoupledFromLiveSpan is the regression test for
// apache/skywalking#13885 ("Service crash caused by Go agent unexpected fault address").
//
// Root cause: TransformSegmentObject used to publish the span's Tags/Logs slices
// by reference ("Tags: s.Tags()"). The produced SegmentObject was then queued and
// marshalled by the gRPC reporter goroutine (the "send queue"). When the same span
// was still being mutated - e.g. the gorm plugin stores one span per *gorm.DB and a
// single *gorm.DB session is reused across goroutines - the reporter walked a slice
// that another goroutine was appending to, corrupting the protobuf message and
// faulting inside MessageInfo.sizePointerSlow.
//
// The fix deep-copies Tags/Logs in TransformSegmentObject so the reported segment
// shares no mutable storage with the live span. This test verifies that guarantee:
// once a segment has been transformed for sending, concurrently mutating the span
// must not race with marshalling the segment.
func TestRaceSegmentDecoupledFromLiveSpan(t *testing.T) {
	span := buildReportedSpan()
	const nTags = 16
	for i := 0; i < nTags; i++ {
		span.Tag(fmt.Sprintf("key-%d", i), "init")
	}

	transform := reporter.NewTransform(&reporter.Entity{
		ServiceName:         "svc",
		ServiceInstanceName: "inst",
	})

	// The reporter transforms the finished segment once and hands it to the send queue.
	seg := transform.TransformSegmentObject([]reporter.ReportedSpan{span})

	var stop int32
	var wg sync.WaitGroup
	wg.Add(2)

	// goroutine A: a concurrent caller keeps mutating the SAME span (shared-session
	// misuse): it both updates existing tag values in place and appends new tags.
	go func() {
		defer wg.Done()
		for i := 0; atomic.LoadInt32(&stop) == 0; i++ {
			span.Tag(fmt.Sprintf("key-%d", i%nTags), fmt.Sprintf("v-%d", i))
			span.Tag(fmt.Sprintf("extra-%d", i), "v")
		}
	}()

	// goroutine B: the send queue marshals the already-transformed segment.
	go func() {
		defer wg.Done()
		for atomic.LoadInt32(&stop) == 0 {
			if _, err := proto.Marshal(seg); err != nil {
				t.Errorf("marshal segment: %v", err)
				return
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)
	atomic.StoreInt32(&stop, 1)
	wg.Wait()
}

// TestRaceReporterNeverPanicsWhileSpanMutated is the panic-focused regression test
// for apache/skywalking#13885. It reproduces the exact crash conditions and asserts
// the reporter never panics/faults.
//
// A finished segment is transformed once (as the agent does when a segment ends),
// then several "send queue" goroutines keep marshalling it while ONE caller keeps
// mutating the original span - in particular updating existing tag values IN PLACE,
// which is the write that, before the fix, let the reporter read a half-overwritten
// string and fault inside the protobuf encoder. A single mutator is used on purpose
// so the test isolates the reporter-vs-span race (the fix's guarantee) rather than
// span-vs-span writer races, which are a separate concern.
//
// Each goroutine recovers from panics and the test fails if any occurred, so a
// regression surfaces as a clean failure instead of crashing the test binary.
func TestRaceReporterNeverPanicsWhileSpanMutated(t *testing.T) {
	span := buildReportedSpan()
	const nTags = 32
	for i := 0; i < nTags; i++ {
		span.Tag(fmt.Sprintf("key-%d", i), "init")
		span.Log("event", fmt.Sprintf("log-%d", i))
	}

	transform := reporter.NewTransform(&reporter.Entity{
		ServiceName:         "svc",
		ServiceInstanceName: "inst",
	})
	seg := transform.TransformSegmentObject([]reporter.ReportedSpan{span})

	var stop int32
	var panicked int32
	var wg sync.WaitGroup

	guard := func(fn func()) {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt32(&panicked, 1)
				t.Errorf("reporter panicked while the span was mutated concurrently: %v", r)
			}
		}()
		fn()
	}

	// one caller keeps mutating the live span: in-place tag-value updates, new tags and logs.
	wg.Add(1)
	go guard(func() {
		for i := 0; atomic.LoadInt32(&stop) == 0; i++ {
			span.Tag(fmt.Sprintf("key-%d", i%nTags), fmt.Sprintf("v-%d", i))
			span.Tag(fmt.Sprintf("extra-%d", i), "v")
			span.Log("k", fmt.Sprintf("v-%d", i))
		}
	})

	// several send-queue goroutines marshal the already-transformed segment.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go guard(func() {
			for atomic.LoadInt32(&stop) == 0 {
				if _, err := proto.Marshal(seg); err != nil {
					t.Errorf("marshal segment: %v", err)
					return
				}
			}
		})
	}

	time.Sleep(500 * time.Millisecond)
	atomic.StoreInt32(&stop, 1)
	wg.Wait()

	if atomic.LoadInt32(&panicked) > 0 {
		t.Fatalf("reporter panicked %d time(s) - the #13885 crash regressed", atomic.LoadInt32(&panicked))
	}
}

// wireCollector attaches a working collector harness (collect channel +
// collectorDone) to the span so End()/AsyncFinish() can run their real end0
// path inside tests. It returns the channel the span will be delivered on.
func wireCollector(t *testing.T, span *SegmentSpanImpl) chan reporter.ReportedSpan {
	ch := make(chan reporter.ReportedSpan, 8)
	done := make(chan struct{})
	span.SegmentContext.collect = ch
	span.SegmentContext.collectorDone = done
	span.DefaultSpan.EndTime = time.Time{} // not ended yet
	span.DefaultSpan.ended = false
	span.DefaultSpan.tracer = Tracing // so11y bookkeeping in End needs a tracer
	t.Cleanup(func() { close(done) }) // release any straggling end0 goroutine
	return ch
}

// TestRaceConcurrentMutators reproduces the "crossed span" misuse (e.g. the gorm
// plugin handing the same live span to two goroutines through a shared
// *gorm.DB): several goroutines mutate ONE span concurrently, repeatedly
// rewriting the same tag key in place - the exact write that used to tear the
// string header read by the reporter. With the per-span lock this must be
// race-detector clean; before the fix this test reports races immediately.
func TestRaceConcurrentMutators(t *testing.T) {
	span := buildReportedSpan()
	const workers = 4
	var stop int32
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; atomic.LoadInt32(&stop) == 0; i++ {
				span.Tag("db.statement", fmt.Sprintf("select %d from t%d", i, w)) // same-key in-place rewrite
				span.Tag(fmt.Sprintf("k-%d-%d", w, i%8), "v")
				span.Log("event", fmt.Sprintf("w%d-%d", w, i))
				span.SetOperationName(fmt.Sprintf("op-%d-%d", w, i))
				span.SetPeer("10.0.0.1:3306")
				span.Error("boom")
			}
		}(w)
	}
	time.Sleep(200 * time.Millisecond)
	atomic.StoreInt32(&stop, 1)
	wg.Wait()
}

// TestRaceMutateAfterFreezeWhileReporting verifies the lock-free reporting
// guarantee: once endAndFreeze returns, the reporter may transform and marshal
// the span without locks even though other goroutines are still calling
// mutators (their writes are dropped under the lock without touching data).
func TestRaceMutateAfterFreezeWhileReporting(t *testing.T) {
	span := buildReportedSpan()
	for i := 0; i < 16; i++ {
		span.Tag(fmt.Sprintf("key-%d", i), "init")
		span.Log("event", fmt.Sprintf("log-%d", i))
	}

	var stop int32
	var wg sync.WaitGroup
	wg.Add(3)
	for w := 0; w < 3; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; atomic.LoadInt32(&stop) == 0; i++ {
				span.Tag(fmt.Sprintf("key-%d", i%16), fmt.Sprintf("late-%d-%d", w, i))
				span.Log("late", "v")
				span.SetOperationName("late-op")
				span.Error("late")
			}
		}(w)
	}

	time.Sleep(50 * time.Millisecond) // let some pre-freeze writes land (race builds are slow)
	if !span.endAndFreeze() {
		t.Fatal("first endAndFreeze must return true")
	}

	transform := reporter.NewTransform(&reporter.Entity{
		ServiceName:         "svc",
		ServiceInstanceName: "inst",
	})
	// 50 iterations: the race detector is a binary signal, more iterations add
	// wall time (~7s -> ~2s under -race) without adding coverage
	for i := 0; i < 50; i++ {
		seg := transform.TransformSegmentObject([]reporter.ReportedSpan{span})
		if seg == nil {
			t.Fatal("nil segment")
		}
		if _, err := proto.Marshal(seg); err != nil {
			t.Fatalf("marshal segment: %v", err)
		}
	}

	atomic.StoreInt32(&stop, 1)
	wg.Wait()
}

// TestRaceDoubleEnd races two End() calls on the same span and asserts the
// segment collector receives the span exactly once (the old IsValid check-then-
// act allowed duplicated end0 sends, corrupting the segment accounting).
func TestRaceDoubleEnd(t *testing.T) {
	ResetTracingContext()
	defer ResetTracingContext()
	span := buildReportedSpan()
	ch := wireCollector(t, span)

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			span.End()
		}()
	}
	wg.Wait()

	received := 0
	select {
	case <-ch:
		received++
	case <-time.After(2 * time.Second):
	}
	// grace period for a (buggy) duplicated delivery
	select {
	case <-ch:
		received++
	case <-time.After(200 * time.Millisecond):
	}
	if received != 1 {
		t.Fatalf("expected the span to be collected exactly once, got %d", received)
	}
}

// TestRaceAsyncFinishVsMutators covers the async span pattern (gRPC streaming,
// toolkit async API): one goroutine keeps tagging while another finishes the
// span asynchronously. Both AsyncFinish/End themselves and the mutators must be
// fully synchronized; before the fix AsyncFinish/End were unlocked even in
// async mode.
func TestRaceAsyncFinishVsMutators(t *testing.T) {
	ResetTracingContext()
	defer ResetTracingContext()
	span := buildReportedSpan()
	ch := wireCollector(t, span)
	span.PrepareAsync()

	var stop int32
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; atomic.LoadInt32(&stop) == 0; i++ {
			span.Tag("async-key", fmt.Sprintf("v-%d", i))
			span.Log("async", "v")
		}
	}()

	span.End() // async mode: does not finish the span

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		span.AsyncFinish()
	}()
	<-finished

	atomic.StoreInt32(&stop, 1)
	wg.Wait()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("async finished span was never collected")
	}
}

// TestRaceHostileWorkload runs the full end-to-end hostile workload (see
// span_hostile_workload_test.go) in-process under the race detector: every
// misuse pattern from the #13885 audit against the real pipeline must be free
// of data races. The no-panic/no-throw property of the same workload is
// asserted by TestE2ESpanCrashSafety in a child process.
func TestRaceHostileWorkload(t *testing.T) {
	segments, marshals := runHostileSpanWorkload(1500 * time.Millisecond)
	if segments == 0 || marshals == 0 {
		t.Fatalf("hostile workload processed no data (segments=%d marshals=%d)", segments, marshals)
	}
}

// TestRaceCorrelationSetVsSnapshot covers the correlation storage: concurrent
// writers and snapshot/encode readers used to race on a bare map, which is an
// unrecoverable "concurrent map iteration and map write" fatal error in
// production.
func TestRaceCorrelationSetVsSnapshot(t *testing.T) {
	c := newCorrelationContext()
	var stop int32
	var wg sync.WaitGroup
	wg.Add(4)
	for w := 0; w < 2; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; atomic.LoadInt32(&stop) == 0; i++ {
				c.Set(fmt.Sprintf("k%d", i%4), fmt.Sprintf("v-%d-%d", w, i))
				if i%8 == 0 {
					c.Set(fmt.Sprintf("k%d", i%4), "") // delete path
				}
			}
		}(w)
	}
	for r := 0; r < 2; r++ {
		go func() {
			defer wg.Done()
			for atomic.LoadInt32(&stop) == 0 {
				_ = c.Snapshot()
				_ = c.Get("k1")
				_ = c.Len()
				_ = c.Clone()
			}
		}()
	}
	time.Sleep(200 * time.Millisecond)
	atomic.StoreInt32(&stop, 1)
	wg.Wait()
}
