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

package socket

import (
	"sync"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// fakeEnhancedInstance stands in for the toolchain-enhanced socket instance.
// The fake synchronizes the field so the test isolates the one-shot guard
// itself; the raciness of the real generated accessor is the known
// toolchain-level dynamic-field issue, not what this test verifies.
type fakeEnhancedInstance struct {
	mu    sync.Mutex
	field interface{}
}

func (f *fakeEnhancedInstance) GetSkyWalkingDynamicField() interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.field
}

func (f *fakeEnhancedInstance) SetSkyWalkingDynamicField(val interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.field = val
}

// TestConcurrentCloseFinishesOnce replicates two racing Close calls on the
// same connection: both read a non-nil dynamic field, but the one-shot guard
// lets only one of them run AsyncFinish (a double AsyncFinish used to panic
// before the core made it drop-and-log; now it cannot happen at all).
func TestConcurrentCloseFinishesOnce(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	span, err := tracing.CreateEntrySpan("micro/connection", func(string) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}
	span.PrepareAsync()
	snapshot := tracing.CaptureContext()
	span.End()

	instance := &fakeEnhancedInstance{field: &InjectData{Span: span, Snapshot: snapshot}}
	interceptor := &CloseInterceptor{}

	const closers = 8
	var wg sync.WaitGroup
	for i := 0; i < closers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := interceptor.AfterInvoke(operator.NewInvocation(instance)); err != nil {
				t.Errorf("close interceptor error: %v", err)
			}
		}()
	}
	wg.Wait()

	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < 1 {
		if time.Now().After(deadline) {
			t.Fatal("connection span was never reported")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := len(core.GetReportedSpans()); got != 1 {
		t.Fatalf("connection span must be reported exactly once, got %d", got)
	}
	if instance.GetSkyWalkingDynamicField() != nil {
		t.Fatal("the winner must clear the dynamic field so a reused socket gets a fresh span")
	}
}

// TestCloseWithoutInjectDataIsNoop keeps the nil/foreign dynamic-field guard.
func TestCloseWithoutInjectDataIsNoop(t *testing.T) {
	interceptor := &CloseInterceptor{}
	if err := interceptor.AfterInvoke(operator.NewInvocation(&fakeEnhancedInstance{})); err != nil {
		t.Fatal(err)
	}
	if err := interceptor.AfterInvoke(operator.NewInvocation(&fakeEnhancedInstance{field: "not-inject-data"})); err != nil {
		t.Fatal(err)
	}
}
