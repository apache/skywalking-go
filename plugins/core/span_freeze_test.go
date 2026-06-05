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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core/reporter"
)

// newTestSegmentSpan builds a SegmentSpanImpl the same way the agent does,
// usable from non-race builds (buildReportedSpan lives behind the race tag).
func newTestSegmentSpan() *SegmentSpanImpl {
	return &SegmentSpanImpl{
		DefaultSpan: DefaultSpan{
			StartTime:     time.Now(),
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

func TestLateWritesDroppedAfterFreeze(t *testing.T) {
	span := newTestSegmentSpan()
	span.Tag("k1", "v1")
	span.Log("event", "before")
	span.SetOperationName("op-before")

	if !span.endAndFreeze() {
		t.Fatal("first endAndFreeze must return true")
	}
	if span.endAndFreeze() {
		t.Fatal("second endAndFreeze must return false")
	}

	// every late mutation must be silently dropped without panicking
	span.Tag("k1", "late")
	span.Tag("k2", "late")
	span.Log("event", "late")
	span.SetOperationName("op-late")
	span.SetPeer("late:1")
	span.SetSpanLayer(int32(3))
	span.SetComponent(99)
	span.Error("late")
	span.ErrorOccured()

	if got := len(span.Tags()); got != 1 {
		t.Fatalf("late tag was not dropped, tags=%d", got)
	}
	if span.Tags()[0].Value != "v1" {
		t.Fatalf("in-place tag rewrite after freeze was not dropped: %s", span.Tags()[0].Value)
	}
	if got := len(span.Logs()); got != 1 {
		t.Fatalf("late log was not dropped, logs=%d", got)
	}
	if span.OperationName() != "op-before" {
		t.Fatalf("late operation name was not dropped: %s", span.OperationName())
	}
	if span.IsError() {
		t.Fatal("late error flag was not dropped")
	}
}

func TestEnd0AfterCollectorExitIsSafe(t *testing.T) {
	ResetTracingContext()
	span := newTestSegmentSpan()
	span.DefaultSpan.tracer = Tracing
	// collector already exited: data channel has no receiver and stays open,
	// only the done channel is closed
	span.SegmentContext.collect = make(chan reporter.ReportedSpan)
	span.SegmentContext.collectorDone = make(chan struct{})
	close(span.SegmentContext.collectorDone)

	span.End() // end0 must neither panic nor block forever

	// the send goroutine selects the done branch; give it a moment and make
	// sure the test itself completes (a blocked send would hang the test)
	time.Sleep(50 * time.Millisecond)
}

func TestDoubleEndCollectsOnce(t *testing.T) {
	ResetTracingContext()
	span := newTestSegmentSpan()
	span.DefaultSpan.tracer = Tracing
	ch := make(chan reporter.ReportedSpan, 8)
	span.SegmentContext.collect = ch
	span.SegmentContext.collectorDone = make(chan struct{})

	span.End()
	span.End() // second End must be a no-op

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("ended span was never collected")
	}
	select {
	case <-ch:
		t.Fatal("span was collected twice")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAsyncFlowCollectsOnce(t *testing.T) {
	ResetTracingContext()
	span := newTestSegmentSpan()
	span.DefaultSpan.tracer = Tracing
	ch := make(chan reporter.ReportedSpan, 8)
	span.SegmentContext.collect = ch
	span.SegmentContext.collectorDone = make(chan struct{})

	span.PrepareAsync()
	span.End() // async mode: must not collect yet
	select {
	case <-ch:
		t.Fatal("async span collected before AsyncFinish")
	case <-time.After(200 * time.Millisecond): // generous window so a slow end0 goroutine could not hide a premature delivery
	}

	span.Tag("after-end", "v") // still allowed between End and AsyncFinish
	span.AsyncFinish()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("async span was never collected")
	}
	found := false
	for _, tag := range span.Tags() {
		if tag.Key == "after-end" {
			found = true
		}
	}
	if !found {
		t.Fatal("tag written between End and AsyncFinish was lost")
	}
}

func TestCorrelationContextBasics(t *testing.T) {
	c := newCorrelationContext()
	c.Set("a", "1")
	c.Set("b", "2")
	if c.Get("a") != "1" || c.Len() != 2 {
		t.Fatal("set/get/len mismatch")
	}
	c.Set("a", "") // empty value deletes
	if c.Get("a") != "" || c.Len() != 1 {
		t.Fatal("empty-value delete failed")
	}

	snap := c.Snapshot()
	clone := c.Clone()
	c.Set("b", "changed")
	if snap["b"] != "2" || clone.Get("b") != "2" {
		t.Fatal("snapshot/clone must be independent of later writes")
	}

	var nilCtx *CorrelationContext
	if nilCtx.Get("x") != "" || nilCtx.Len() != 0 {
		t.Fatal("nil receiver reads must be safe")
	}
	nilCtx.Set("x", "y") // must not panic
	if nilCtx.Clone() == nil {
		t.Fatal("nil receiver clone must return a usable value")
	}
	// Snapshot returns nil when empty (allocation-free); nil maps are readable
	if len(nilCtx.Snapshot()) != 0 {
		t.Fatal("nil receiver snapshot must be empty")
	}

	// lazy data allocation: a fresh context must support all reads before any Set
	fresh := newCorrelationContext()
	if fresh.Get("x") != "" || fresh.Len() != 0 || fresh.Snapshot() != nil {
		t.Fatal("fresh context reads must be safe and allocation-free")
	}
	fresh.Set("x", "1")
	if fresh.Get("x") != "1" {
		t.Fatal("set after lazy init failed")
	}
}

// TestContinueContextClonesRuntime guards the clone-on-continue behavior: two
// goroutines continuing the same snapshot must not share one RuntimeContext map
// (that sharing was a fatal concurrent-map-access risk, e.g. the send and
// receive goroutines of a gRPC stream).
func TestContinueContextClonesRuntime(t *testing.T) {
	ResetTracingContext()
	defer ResetTracingContext()

	Tracing.SetRuntimeContextValue("k", "original")
	snap := Tracing.CaptureContext()
	if snap == nil {
		t.Fatal("capture returned nil snapshot")
	}

	Tracing.ContinueContext(snap)
	Tracing.SetRuntimeContextValue("k", "changed-after-first-continue")

	Tracing.ContinueContext(snap) // the same snapshot continued again
	if got := Tracing.GetRuntimeContextValue("k"); got != "original" {
		t.Fatalf("ContinueContext shared the runtime map across continues: got %v", got)
	}
}

// TestMetricsCollectPanicRecovered proves the metrics collect loop survives a
// panicking user meter callback (it used to kill the whole process: the
// goroutine had no recover).
// The metrics collect goroutine leaks by design (the loop has no shutdown) and
// keeps reading the MetricsObtain global forever - ANY later write to that
// global (restore on test exit, or re-install on `-test.count=2` reruns) would
// race with those leaked readers. So the panicking wrapper is installed exactly
// once per process, before the first collect goroutine exists (happens-before
// safe), and reruns only flip the atomic switch.
var (
	metricsPanicHookOnce sync.Once
	metricsPanicArmed    atomic.Bool
	metricsPanicCalls    atomic.Int32
)

func TestMetricsCollectPanicRecovered(t *testing.T) {
	metricsPanicHookOnce.Do(func() {
		old := MetricsObtain
		MetricsObtain = func() ([]interface{}, []func()) {
			if !metricsPanicArmed.Load() {
				return old()
			}
			metricsPanicCalls.Add(1)
			panic("meter callback boom")
		}
	})
	metricsPanicCalls.Store(0)
	metricsPanicArmed.Store(true)
	defer metricsPanicArmed.Store(false)

	// A dedicated tracer keeps the leaked collect goroutine (the loop has no
	// shutdown mechanism) away from the shared Tracing used by other tests.
	tr := &Tracer{initFlag: 1, Sampler: NewConstSampler(true), Reporter: NewStoreReporter(),
		ServiceEntity: NewEntity("metrics-recover-test", "inst"), meterMap: &sync.Map{}}
	tr.ProfileManager = NewProfileManager(nil)
	tr.initMetricsCollect(1)

	deadline := time.After(5 * time.Second)
	for metricsPanicCalls.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("collect loop did not survive the panic, iterations=%d", metricsPanicCalls.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}
}
