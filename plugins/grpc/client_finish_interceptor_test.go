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
// lifecycle (PrepareAsync + End in the streaming interceptor, finish armed by
// RecvMsg) and lets several Finish calls race: exactly one may consume the
// flag and run AsyncFinish, and the span must be reported exactly once.
// The flag used to be a plain bool, which was both a data race (RecvMsg
// goroutine vs gRPC-internal Finish goroutine) and a double-AsyncFinish risk.
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
	data.interceptFinish.Store(true) // RecvMsg armed the finish

	const finishers = 8
	var wg sync.WaitGroup
	winners := make(chan bool, finishers)
	for i := 0; i < finishers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			winners <- finishStreamSpan(data)
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
		t.Fatalf("exactly one Finish may run AsyncFinish, got %d winners", winnerCount)
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

// TestFinishWithoutRecvMsgIsNoop pins the unarmed case.
func TestFinishWithoutRecvMsgIsNoop(t *testing.T) {
	data := &contextData{}
	if finishStreamSpan(data) {
		t.Fatal("finish without a RecvMsg must not consume anything")
	}
}
