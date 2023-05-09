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

package reporter

import (
	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

// Tag are supported by sky-walking engine.
// As default, all Tags will be stored, but these ones have
// particular meanings.
type Tag string

// SegmentContext is the context in a segment
type SegmentContext interface {
	GetTraceID() string
	GetSegmentID() string
	GetSpanID() int32
	GetParentSpanID() int32
	GetParentSegmentID() string
}

// SpanContext defines propagation specification of SkyWalking
type SpanContext interface {
	GetTraceID() string
	GetParentSegmentID() string
	GetParentService() string
	GetParentServiceInstance() string
	GetParentEndpoint() string
	GetAddressUsedAtClient() string
	GetParentSpanID() int32
}

type ReportedSpan interface {
	Context() SegmentContext
	Refs() []SpanContext
	StartTime() int64
	EndTime() int64
	OperationName() string
	Peer() string
	SpanType() agentv3.SpanType
	SpanLayer() agentv3.SpanLayer
	IsError() bool
	Tags() []*commonv3.KeyStringValuePair
	Logs() []*agentv3.Log
	ComponentID() int32
}

type Entity struct {
	ServiceName         string
	ServiceInstanceName string
	Props               []*commonv3.KeyStringValuePair
}

func (e *Entity) GetServiceName() string {
	return e.ServiceName
}

func (e *Entity) GetInstanceName() string {
	return e.ServiceInstanceName
}

type Reporter interface {
	Boot(entity *Entity, cdsWatchers []AgentConfigChangeWatcher)
	Send(spans []ReportedSpan)
	Close()
}
