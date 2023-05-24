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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateID(t *testing.T) {
	ctx := NewTracingContext()
	id, err := GenerateGlobalID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, "", id, "id should not be empty")
}

func BenchmarkGenerateID(b *testing.B) {
	context := NewTracingContext()
	for i := 0; i < b.N; i++ {
		_, err := GenerateGlobalID(context)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateIDParallels(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		context := NewTracingContext()
		for pb.Next() {
			_, err := GenerateGlobalID(context)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkUUID(t *testing.B) {
	for i := 0; i < t.N; i++ {
		_, err := UUID()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkUUIDParallels(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := UUID()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
