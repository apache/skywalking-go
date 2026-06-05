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

// span_hostile_workload_test.go drives the REAL agent pipeline (public tracing
// API -> spans/segments -> collector goroutines -> Transform -> proto.Marshal)
// under every concurrency-abuse pattern found in the apache/skywalking#13885
// investigation, plus aggressive GC and stack growth so that any pointer
// corruption is caught by the runtime scanners (which is exactly how the
// production crash manifested as `fatal error: invalid pointer found on
// stack`).
//
// It is shared by two tests:
//   - TestE2ESpanCrashSafety (span_crash_e2e_test.go): runs the workload in a
//     CHILD PROCESS - a runtime.throw is unrecoverable and kills the process,
//     so only a subprocess can assert "no panic AND no runtime fatal at all";
//   - TestRaceHostileWorkload (segment_datarace_test.go, race build): runs it
//     in-process under the race detector to catch any remaining data race.
//
// There is deliberately NO recover anywhere in this workload or its pipeline
// reporter: any panic must surface and fail the test.
package core

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
	logv3 "github.com/apache/skywalking-go/protocols/collect/logging/v3"
)

// ---------------------------------------------------------------------------
// real goroutine-local GLS for tests
// ---------------------------------------------------------------------------

// goid parses the current goroutine id from the stack header ("goroutine N [").
// Slow, but faithful: it gives the tests true goroutine-local storage, matching
// the production GLS that the build toolchain injects into runtime.g.
func goid() uint64 {
	var buf [32]byte
	n := runtime.Stack(buf[:], false)
	id := uint64(0)
	for _, c := range buf[10:n] {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + uint64(c-'0')
	}
	return id
}

// installGoroutineLocalGLS replaces the single-variable test GLS from
// test_base.go with a real per-goroutine implementation so that context
// propagation (capture/continue, snapshots) behaves like production.
func installGoroutineLocalGLS() (restore func()) {
	oldGet, oldSet := GetGLS, SetGLS
	var m sync.Map
	GetGLS = func() interface{} {
		if v, ok := m.Load(goid()); ok {
			return v
		}
		return nil
	}
	SetGLS = func(v interface{}) {
		if v == nil {
			m.Delete(goid())
			return
		}
		m.Store(goid(), v)
	}
	return func() {
		GetGLS = oldGet
		SetGLS = oldSet
	}
}

// ---------------------------------------------------------------------------
// pipeline reporter: replicates the production gRPC reporter data path
// ---------------------------------------------------------------------------

// pipelineReporter mirrors what the production reporters do with a finished
// segment: TransformSegmentObject on the segment-collector goroutine, then
// proto.Marshal on a dedicated send goroutine. Deliberately NO recover: a
// panic anywhere in this path must crash the test/child process.
type pipelineReporter struct {
	transform *reporter.Transform
	segCh     chan *agentv3.SegmentObject
	closed    chan struct{} // shutdown signal; the DATA channel is never closed
	done      chan struct{}
	segments  int64
	marshals  int64
}

func newPipelineReporter() *pipelineReporter {
	r := &pipelineReporter{
		transform: reporter.NewTransform(&reporter.Entity{ServiceName: "e2e", ServiceInstanceName: "inst"}),
		segCh:     make(chan *agentv3.SegmentObject, 1024),
		closed:    make(chan struct{}),
		done:      make(chan struct{}),
	}
	go func() { // the "send goroutine" of the production pipeline
		defer close(r.done)
		for {
			select {
			case seg := <-r.segCh:
				if _, err := proto.Marshal(seg); err == nil {
					atomic.AddInt64(&r.marshals, 1)
				}
			case <-r.closed:
				for { // drain what is already buffered, then exit
					select {
					case seg := <-r.segCh:
						if _, err := proto.Marshal(seg); err == nil {
							atomic.AddInt64(&r.marshals, 1)
						}
					default:
						return
					}
				}
			}
		}
	}()
	return r
}

func (r *pipelineReporter) SendTracing(spans []reporter.ReportedSpan) {
	// runs on the segment collector goroutine, exactly like production
	seg := r.transform.TransformSegmentObject(spans)
	if seg == nil {
		return
	}
	atomic.AddInt64(&r.segments, 1)
	// Same done-channel pattern as the production fix: straggler collector
	// goroutines may still deliver after the workload ended, and closing the
	// DATA channel here would be exactly the send-on-closed-channel bug this
	// PR eliminates (the race variant of this test caught that mistake in an
	// earlier revision of this harness).
	select {
	case r.segCh <- seg:
	case <-r.closed:
	}
}

func (r *pipelineReporter) closeAndWait() {
	close(r.closed)
	<-r.done
}

func (r *pipelineReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
}
func (r *pipelineReporter) SendMetrics(metrics []reporter.ReportedMeter)        {}
func (r *pipelineReporter) SendLog(log *logv3.LogData)                          {}
func (r *pipelineReporter) ConnectionStatus() reporter.ConnectionStatus        { return reporter.ConnectionStatusConnected }
func (r *pipelineReporter) Close()                                             {}
func (r *pipelineReporter) AddProfileTaskManager(p reporter.ProfileTaskManager) {}

// ---------------------------------------------------------------------------
// hostile flows: one per misuse pattern found in the audit
// ---------------------------------------------------------------------------

// growStack forces stack growth (morestack/copystack) with live span data on
// the stack - the exact runtime path that detected the corrupted pointer in
// the production crash.
//
//go:noinline
func growStack(depth int) int {
	if depth == 0 {
		return 0
	}
	var pad [64]byte
	pad[0] = byte(depth)
	return int(pad[0]) + growStack(depth-1)
}

// wellBehavedFlow is the correct usage baseline: a full entry/exit/local span
// tree flowing through the real pipeline.
func wellBehavedFlow(i int) {
	entry, err := tracing.CreateEntrySpan("GET:/e2e", func(string) (string, error) { return "", nil })
	if err != nil || entry == nil {
		return
	}
	entry.Tag("http.method", "GET")
	if exit, err := tracing.CreateExitSpan("e2e/db", "127.0.0.1:3306",
		func(k, v string) error { return nil }); err == nil && exit != nil {
		exit.Tag("db.type", "sql")
		exit.Tag("db.type", "mysql") // same-key in-place rewrite path
		exit.Log("event", "query")
		exit.End()
	}
	if local, err := tracing.CreateLocalSpan("e2e/biz"); err == nil && local != nil {
		local.Error("boom")
		local.End()
	}
	growStack(64 + i%64)
	entry.End()
	tracing.CleanContext()
}

// crossedSpanFlow reproduces the gorm "crossed span" bug class: one live span
// mutated and ended by several goroutines at once.
func crossedSpanFlow(i int) {
	span, err := tracing.CreateLocalSpan("e2e/crossed")
	if err != nil || span == nil {
		return
	}
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := 0; k < 16; k++ {
				span.Tag("db.statement", fmt.Sprintf("select %d from t%d_%d", k, w, i))
				span.Log("event", "x")
				span.SetOperationName(fmt.Sprintf("op-%d-%d", w, k))
				span.SetPeer("10.0.0.1:3306")
				growStack(32)
			}
			span.End() // partners race End as well
		}(w)
	}
	span.Tag("db.statement", "owner")
	span.End()
	wg.Wait()
	tracing.CleanContext()
}

// lateWriteFlow reproduces the rocketmq/pulsar callback bug class: writes
// arriving after the span was ended and reported.
func lateWriteFlow(i int) {
	span, err := tracing.CreateLocalSpan("e2e/late")
	if err != nil || span == nil {
		return
	}
	span.Tag("k", "v")
	span.End()
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := 0; k < 8; k++ {
				span.Tag("k", fmt.Sprintf("late-%d-%d-%d", w, k, i)) // must be dropped
				span.Error("late")
				span.Log("late", "x")
			}
		}(w)
	}
	wg.Wait()
	tracing.CleanContext()
}

// asyncFlow exercises the documented async pattern (gRPC streaming, toolkit)
// plus late writes after AsyncFinish.
func asyncFlow(i int) {
	span, err := tracing.CreateLocalSpan("e2e/async")
	if err != nil || span == nil {
		return
	}
	span.PrepareAsync()
	span.End()
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := 0; k < 8; k++ {
				span.Tag("async-key", fmt.Sprintf("v-%d-%d-%d", w, k, i))
				span.Log("async", "x")
			}
		}(w)
	}
	wg.Wait()
	done := make(chan struct{})
	go func() {
		defer close(done)
		span.AsyncFinish()
		span.Tag("after-finish", "must-be-dropped")
	}()
	<-done
	tracing.CleanContext()
}

// correlationFlow hammers the correlation store from concurrently continued
// snapshots while exit spans encode propagation headers (the C1/C2 class of
// unrecoverable concurrent-map fatals).
func correlationFlow(i int) {
	root, err := tracing.CreateLocalSpan("e2e/correlation-root")
	if err != nil || root == nil {
		return
	}
	tracing.SetCorrelationContextValue("seed", fmt.Sprintf("%d", i))
	snap := tracing.CaptureContext()
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			defer tracing.CleanContext()
			tracing.ContinueContext(snap)
			for k := 0; k < 8; k++ {
				tracing.SetCorrelationContextValue(fmt.Sprintf("k%d", k%4), fmt.Sprintf("v-%d-%d", w, k))
				_ = tracing.GetCorrelationContextValue("k1")
				if child, err := tracing.CreateExitSpan("e2e/exit", "127.0.0.1:80",
					func(k, v string) error { return nil }); err == nil && child != nil {
					child.Tag("http.method", "GET")
					child.End()
				}
			}
		}(w)
	}
	wg.Wait()
	root.End()
	tracing.CleanContext()
}

// doubleEndFlow races End() on one span from two goroutines.
func doubleEndFlow(i int) {
	span, err := tracing.CreateLocalSpan("e2e/double-end")
	if err != nil || span == nil {
		return
	}
	span.Tag("i", fmt.Sprintf("%d", i))
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			span.End()
		}()
	}
	wg.Wait()
	tracing.CleanContext()
}

// snapshotChaosFlow continues ONE snapshot from several goroutines at once
// (the shared snapshot + runtime-context class: gRPC stream send/recv,
// microv4 connections).
func snapshotChaosFlow(i int) {
	root, err := tracing.CreateLocalSpan("e2e/snapshot-root")
	if err != nil || root == nil {
		return
	}
	snap := tracing.CaptureContext()
	var wg sync.WaitGroup
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			defer tracing.CleanContext()
			tracing.ContinueContext(snap)
			if child, err := tracing.CreateLocalSpan(fmt.Sprintf("e2e/snap-child-%d", w)); err == nil && child != nil {
				child.Tag("w", fmt.Sprintf("%d-%d", w, i))
				child.Log("event", "child")
				child.End()
			}
		}(w)
	}
	wg.Wait()
	root.End()
	tracing.CleanContext()
}

// ---------------------------------------------------------------------------
// the workload driver
// ---------------------------------------------------------------------------

// runHostileSpanWorkload runs every hostile flow concurrently against the real
// pipeline for roughly d, under aggressive GC. It returns the number of
// segments transformed and payloads marshalled so callers can assert the
// pipeline actually processed work.
func runHostileSpanWorkload(d time.Duration) (segments, marshals int64) {
	restore := installGoroutineLocalGLS()
	defer restore()
	ResetTracingContext()
	rep := newPipelineReporter()
	Tracing.Reporter = rep

	// aggressive GC maximizes heap/stack scans - the runtime checks that turn
	// any pointer corruption into `fatal error: ... bad pointer ...`
	oldGC := debug.SetGCPercent(10)
	defer debug.SetGCPercent(oldGC)

	deadline := time.Now().Add(d)
	var wg sync.WaitGroup
	launch := func(n int, fn func(i int)) {
		for w := 0; w < n; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer tracing.CleanContext()
				for i := 0; time.Now().Before(deadline); i++ {
					fn(i)
				}
			}()
		}
	}

	launch(2, wellBehavedFlow)
	launch(2, crossedSpanFlow)
	launch(1, lateWriteFlow)
	launch(1, asyncFlow)
	launch(1, correlationFlow)
	launch(1, doubleEndFlow)
	launch(1, snapshotChaosFlow)

	wg.Wait()
	time.Sleep(200 * time.Millisecond) // let collector goroutines flush
	rep.closeAndWait()
	ResetTracingContext()
	return atomic.LoadInt64(&rep.segments), atomic.LoadInt64(&rep.marshals)
}
