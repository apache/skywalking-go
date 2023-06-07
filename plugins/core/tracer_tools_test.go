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

	"github.com/apache/skywalking-go/plugins/core/tools"
)

type testInterface interface {
	TestMethod()
}

type testStruct struct {
	key1 string
	key2 testInterface
	key3 *testStruct
	Key4 string
}

func (t *testStruct) TestMethod() {
}

func TestReflectGetValue(t *testing.T) {
	var testInstance = &testStruct{key1: "testValue"}
	tests := []struct {
		instance interface{}
		filter   []tools.ReflectFieldFilter
		result   interface{}
	}{
		{
			instance: &testStruct{key1: "test1"},
			filter: []tools.ReflectFieldFilter{
				tools.WithFieldName("key1"),
				tools.WithType(""),
			},
			result: "test1",
		},
		{
			instance: testStruct{key1: "test1"},
			filter: []tools.ReflectFieldFilter{
				tools.WithFieldName("key1"),
				tools.WithType(""),
			},
			result: nil,
		},
		{
			instance: &testStruct{key2: testInstance},
			filter: []tools.ReflectFieldFilter{
				tools.WithFieldName("key2"),
				tools.WithInterfaceType((*testInterface)(nil)),
			},
			result: testInstance,
		},
		{
			instance: &testStruct{key3: testInstance},
			filter: []tools.ReflectFieldFilter{
				tools.WithFieldName("key3"),
				tools.WithType(testInstance),
			},
			result: testInstance,
		},
		{
			instance: &testStruct{Key4: "test"},
			filter: []tools.ReflectFieldFilter{
				tools.WithFieldName("Key4"),
			},
			result: "test",
		},
	}

	for inx, tt := range tests {
		result := tools.GetInstanceValueByType(tt.instance, tt.filter...)
		if result != tt.result {
			t.Errorf("test %d: expect %v, actual %v", inx, tt.result, result)
		}
	}
}
