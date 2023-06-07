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
	"math"
	"sync"
	"time"

	"github.com/apache/skywalking-go/plugins/core/reporter"

	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type DefaultSpan struct {
	Refs          []reporter.SpanContext
	tracer        *Tracer
	StartTime     time.Time
	EndTime       time.Time
	OperationName string
	Peer          string
	Layer         agentv3.SpanLayer
	ComponentID   int32
	Tags          []*commonv3.KeyStringValuePair
	Logs          []*agentv3.Log
	IsError       bool
	SpanType      SpanType
	Parent        TracingSpan

	InAsyncMode       bool
	AsyncModeFinished bool
	AsyncOpLocker     *sync.Mutex
}

func NewDefaultSpan(tracer *Tracer, parent TracingSpan) *DefaultSpan {
	return &DefaultSpan{
		tracer:    tracer,
		StartTime: time.Now(),
		SpanType:  SpanTypeLocal,
		Parent:    parent,
	}
}

// For TracingSpan
func (ds *DefaultSpan) SetOperationName(name string) {
	if ds.InAsyncMode {
		ds.AsyncOpLocker.Lock()
		defer ds.AsyncOpLocker.Unlock()
	}
	ds.OperationName = name
}

func (ds *DefaultSpan) GetOperationName() string {
	return ds.OperationName
}

func (ds *DefaultSpan) SetPeer(peer string) {
	if ds.InAsyncMode {
		ds.AsyncOpLocker.Lock()
		defer ds.AsyncOpLocker.Unlock()
	}
	ds.Peer = peer
}

func (ds *DefaultSpan) GetPeer() string {
	return ds.Peer
}

func (ds *DefaultSpan) SetSpanLayer(layer int32) {
	if ds.InAsyncMode {
		ds.AsyncOpLocker.Lock()
		defer ds.AsyncOpLocker.Unlock()
	}
	ds.Layer = agentv3.SpanLayer(layer)
}

func (ds *DefaultSpan) GetSpanLayer() agentv3.SpanLayer {
	return ds.Layer
}

func (ds *DefaultSpan) SetComponent(componentID int32) {
	if ds.InAsyncMode {
		ds.AsyncOpLocker.Lock()
		defer ds.AsyncOpLocker.Unlock()
	}
	ds.ComponentID = componentID
}

func (ds *DefaultSpan) GetComponent() int32 {
	return ds.ComponentID
}

func (ds *DefaultSpan) Tag(key, value string) {
	if ds.InAsyncMode {
		ds.AsyncOpLocker.Lock()
		defer ds.AsyncOpLocker.Unlock()
	}
	for _, tag := range ds.Tags {
		if tag.Key == key {
			tag.Value = value
			return
		}
	}
	ds.Tags = append(ds.Tags, &commonv3.KeyStringValuePair{Key: key, Value: value})
}

func (ds *DefaultSpan) Log(ll ...string) {
	if ds.InAsyncMode {
		ds.AsyncOpLocker.Lock()
		defer ds.AsyncOpLocker.Unlock()
	}
	data := make([]*commonv3.KeyStringValuePair, 0, int32(math.Ceil(float64(len(ll))/2.0)))
	var kvp *commonv3.KeyStringValuePair
	for i, l := range ll {
		if i%2 == 0 {
			kvp = &commonv3.KeyStringValuePair{}
			data = append(data, kvp)
			kvp.Key = l
		} else if kvp != nil {
			kvp.Value = l
		}
	}
	ds.Logs = append(ds.Logs, &agentv3.Log{Time: Millisecond(time.Now()), Data: data})
}

func (ds *DefaultSpan) Error(ll ...string) {
	if ds.InAsyncMode {
		ds.AsyncOpLocker.Lock()
		defer ds.AsyncOpLocker.Unlock()
	}
	ds.IsError = true
	ds.Log(ll...)
}

func (ds *DefaultSpan) End(changeParent bool) {
	ds.EndTime = time.Now()
	if changeParent {
		if ctx := getTracingContext(); ctx != nil {
			ctx.SaveActiveSpan(ds.Parent)
		}
	}
}

func (ds *DefaultSpan) IsEntry() bool {
	return ds.SpanType == SpanTypeEntry
}

func (ds *DefaultSpan) IsExit() bool {
	return ds.SpanType == SpanTypeExit
}

func (ds *DefaultSpan) IsValid() bool {
	return ds.EndTime.IsZero()
}

func (ds *DefaultSpan) ParentSpan() TracingSpan {
	return ds.Parent
}

func (ds *DefaultSpan) PrepareAsync() {
	if ds.InAsyncMode {
		panic("already in async mode")
	}
	ds.InAsyncMode = true
	ds.AsyncModeFinished = false
	ds.AsyncOpLocker = &sync.Mutex{}
}

func (ds *DefaultSpan) AsyncFinish() {
	if !ds.InAsyncMode {
		panic("not in async mode")
	}
	if ds.AsyncModeFinished {
		panic("already finished async")
	}
	ds.AsyncModeFinished = true
}
