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
	"testing"
)

var samplerOperationName = "op"

func TestConstSampler_IsSampled(t *testing.T) {
	sampler := NewConstSampler(true)
	sampled := sampler.IsSampled(samplerOperationName)
	if sampled != true {
		t.Errorf("const sampler should be sampled")
	}
	samplerNegative := NewConstSampler(false)
	sampledNegative := samplerNegative.IsSampled(samplerOperationName)
	if sampledNegative != false {
		t.Errorf("const sampler should not be sampled")
	}
}

func TestRandomSampler_IsSampled(t *testing.T) {
	randomSampler := NewRandomSampler(0.5)

	t.Run("threshold need transform", func(t *testing.T) {
		if randomSampler.threshold != 50 {
			t.Errorf("threshold should be 50")
		}
	})

	// just for test case
	randomSampler.threshold = 100
	sampled := randomSampler.IsSampled(samplerOperationName)
	if sampled != true {
		t.Errorf("random sampler should be sampled")
	}

	randomSampler.threshold = 0
	sampled = randomSampler.IsSampled(samplerOperationName)
	if sampled != false {
		t.Errorf("random sampler should not be sampled")
	}
}

func TestNewRandomSampler(t *testing.T) {
	randomSampler := NewRandomSampler(100)
	sampled := randomSampler.IsSampled(samplerOperationName)
	if sampled != true {
		t.Errorf("random sampler should be sampled")
	}
}

func TestRandomSampler_getRandomizer(t *testing.T) {
	t.Run("must not nil", func(t *testing.T) {
		sampler := &RandomSampler{
			pool: sync.Pool{},
		}

		if sampler.getRandomizer() == nil {
			t.Errorf("randomizer should be nil")
		}
	})

	t.Run("must not nil, if got not a *rand.Rand", func(t *testing.T) {
		sampler := &RandomSampler{
			pool: sync.Pool{},
		}

		sampler.pool.Put(&struct{}{})
		if sampler.getRandomizer() == nil {
			t.Errorf("randomizer should be nil")
		}
	})
}

func BenchmarkRandomPoolSampler_IsSampled(b *testing.B) {
	sampler := NewRandomSampler(0.5)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sampler.IsSampled(samplerOperationName)
		}
	})
}
