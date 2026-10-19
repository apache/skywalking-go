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

package grpc

import (
	"sync"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// TestConcurrentFinishRunsAsyncFinishOnce replicates the streaming client
// lifecycle (PrepareAsync + End when NewStream returns) and lets several
// finishes race: exactly one may finish the span, and the span must be
// reported exactly once.
func TestConcurrentFinishRunsAsyncFinishOnce(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := tracing.CreateExitSpan("/grpc.TestService/Streaming", "localhost:9000",
		func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	span.PrepareAsync()
	span.End()

	data := &contextData{asyncSpan: span}
	gen, _ := data.claim()
	if finished, _ := data.started(); finished {
		t.Fatal("a stream that has not finished must not be finished when NewStream returns")
	}

	const finishers = 8
	var wg sync.WaitGroup
	winners := make(chan bool, finishers)
	for i := 0; i < finishers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			won := data.finish(gen, nil)
			if won {
				finishAsyncSpan(span, nil)
			}
			winners <- won
		}()
	}
	wg.Wait()
	close(winners)

	winnerCount := 0
	for won := range winners {
		if won {
			winnerCount++
		}
	}
	if winnerCount != 1 {
		t.Fatalf("exactly one finish may run AsyncFinish, got %d winners", winnerCount)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < 1 {
		if time.Now().After(deadline) {
			t.Fatal("async stream span was never reported")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := len(core.GetReportedSpans()); got != 1 {
		t.Fatalf("span must be reported exactly once, got %d", got)
	}
}

// TestFinishWithoutStreamSpanIsNoop pins the case of a clientStream without a
// stream span.
func TestFinishWithoutStreamSpanIsNoop(t *testing.T) {
	data := &contextData{}
	finishStreamSpan(data, nil)
	if finished, _ := data.started(); finished {
		t.Fatal("finish without a stream span must not record anything")
	}
}

// TestRaceSnapshotsArePublishedSafely reads the stream snapshots while NewStream
// publishes them, as a stream interceptor that uses the stream on another
// goroutine before NewStream returns can. Run with -race.
func TestRaceSnapshotsArePublishedSafely(t *testing.T) {
	data := &contextData{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = data.continueSnapshot()
			_ = data.endSnapshot()
		}
	}()
	data.setSnapshots(nil, nil)
	wg.Wait()
}
