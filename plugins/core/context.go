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
	"reflect"
	"sync"
)

var (
	GetGLS            = func() interface{} { return nil }
	SetGLS            = func(interface{}) {}
	SetGlobalOperator = func(interface{}) {}
	GetGlobalOperator = func() interface{} { return nil }
	GetInitNotify     = func() []func() { return nil }
	MetricsObtain     = func() ([]interface{}, []func()) { return nil, nil }
)

type ContextSnapshoter interface {
	TakeSnapShot() interface{}
}

type TracingContext struct {
	activeSpan TracingSpan
	Runtime    *RuntimeContext
	ID         *IDContext

	activeSpanLock sync.RWMutex
}

func (t *TracingContext) TakeSnapShot() interface{} {
	if t == nil {
		return nil
	}
	snapshot := newSnapshotSpan(t.ActiveSpan())
	return &TracingContext{
		activeSpan: snapshot,
		Runtime:    t.Runtime.clone(),
		ID:         NewIDContext(false),
	}
}

func (t *TracingContext) ActiveSpan() TracingSpan {
	t.activeSpanLock.RLock()
	defer t.activeSpanLock.RUnlock()
	if t.activeSpan == nil || reflect.ValueOf(t.activeSpan).IsZero() {
		return nil
	}
	return t.activeSpan
}

func (t *TracingContext) SaveActiveSpan(s TracingSpan) {
	t.activeSpanLock.Lock()
	defer t.activeSpanLock.Unlock()
	t.activeSpan = s
}

func (t *TracingContext) RuntimeContext() *RuntimeContext {
	return t.Runtime
}

type RuntimeContext struct {
	data map[string]interface{}
}

func NewTracingContext() *TracingContext {
	return &TracingContext{
		Runtime: &RuntimeContext{
			data: make(map[string]interface{}),
		},
		ID: NewIDContext(true),
	}
}

func (r *RuntimeContext) clone() *RuntimeContext {
	newData := make(map[string]interface{})
	for k, v := range r.data {
		newData[k] = v
	}
	return &RuntimeContext{
		data: newData,
	}
}

func (r *RuntimeContext) Get(key string) interface{} {
	return r.data[key]
}

func (r *RuntimeContext) Set(key string, value interface{}) {
	if value == nil {
		delete(r.data, key)
		return
	}
	r.data[key] = value
}
