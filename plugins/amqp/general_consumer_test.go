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

package amqp

import (
	"fmt"
	"sync"
	"testing"
)

// TestConsumerQueueMappingConcurrency hammers the consumer-tag mapping from
// the three goroutine roles that touch it in production: Consume registers on
// the user goroutine, the delivery dispatch reads on the SDK goroutine and
// Close deletes. The mapping used to be a plain map, which is a fatal
// (unrecoverable) concurrent map read/write.
func TestConsumerQueueMappingConcurrency(t *testing.T) {
	const workers = 8
	const iterations = 500

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		tag := fmt.Sprintf("ctag-%d", w)
		wg.Add(3)
		go func() { // Consume path
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				registerConsumerQueue(tag, "queue-A")
			}
		}()
		go func() { // delivery dispatch path
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if q := consumerQueue(tag); q != "" && q != "queue-A" {
					t.Errorf("unexpected queue %q", q)
				}
			}
		}()
		go func() { // Close path
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				removeConsumerQueue(tag)
			}
		}()
	}
	wg.Wait()
}

// TestConsumerQueueMappingBasics pins the accessor semantics.
func TestConsumerQueueMappingBasics(t *testing.T) {
	registerConsumerQueue("tag-1", "orders")
	if q := consumerQueue("tag-1"); q != "orders" {
		t.Fatalf("expected orders, got %q", q)
	}
	if q := consumerQueue("missing"); q != "" {
		t.Fatalf("missing tag must yield empty queue, got %q", q)
	}
	removeConsumerQueue("tag-1")
	if q := consumerQueue("tag-1"); q != "" {
		t.Fatalf("removed tag must yield empty queue, got %q", q)
	}
}
