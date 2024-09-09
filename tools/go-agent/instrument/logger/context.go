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

package logger

import (
	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
)

var GetOperator = func() Operator { return nil }
var ChangeLogger = func(v interface{}) {}

const noopContextValue = "N/A"

type Operator interface {
	Tracing() interface{}
	ChangeLogger(logger interface{})
	Entity() interface{}
	LogReporter() interface{}
}

type TracingOperator interface {
	ActiveSpan() interface{}
}

type TracingSpan interface {
	GetTraceID() string
	GetSegmentID() string
	GetSpanID() int32
	GetEndPointName() string
	GetParentSpan() interface{}
}

type Entity interface {
	GetServiceName() string
	GetInstanceName() string
}

type NoopSpan struct {
}

func (span *NoopSpan) GetTraceID() string {
	return noopContextValue
}

func (span *NoopSpan) GetSegmentID() string {
	return noopContextValue
}

func (span *NoopSpan) GetSpanID() int32 {
	return -1
}

func (span *NoopSpan) GetParentSpan() interface{} {
	return nil
}

func (span *NoopSpan) GetEndPointName() string {
	return ""
}

func GetLogContext(withEndpoint bool) *core.SkyWalkingLogContext {
	logReporter, ok := GetOperator().LogReporter().(operator.LogReporter)
	if !ok || logReporter == nil {
		return nil
	}

	ctx := logReporter.GetLogContext(withEndpoint)
	if ctx == nil {
		return nil
	}
	return ctx.(*core.SkyWalkingLogContext)
}

func GetLogContextString() string {
	return GetLogContext(false).String()
}
