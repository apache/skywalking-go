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
		},
		SegmentContext: SegmentContext{
			TraceID:      "trace-id",
			SegmentID:    "segment-id",
			SpanID:       0,
			ParentSpanID: -1,
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
