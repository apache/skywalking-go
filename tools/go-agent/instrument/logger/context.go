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

import "fmt"

var GetOperator = func() Operator { return nil }
var ChangeLogger = func(v interface{}) {}

const noopContextValue = "Noop"

type Operator interface {
	Tracing() interface{}
	ChangeLogger(logger interface{})
}

type TracingOperator interface {
	ActiveSpan() interface{}
}

type TracingSpan interface {
	GetTraceID() string
	GetSegmentID() string
	GetSpanID() int32
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
	return 0
}

type SkyWalkingLogContext struct {
	TraceID        string
	TraceSegmentID string
	SpanID         int32
}

var noopContext = &NoopSpan{}

func GetLogContext() *SkyWalkingLogContext {
	operator := GetOperator()
	var activeSpan TracingSpan = noopContext
	if operator != nil {
		tracingOperator := operator.Tracing().(TracingOperator)
		if s, ok := tracingOperator.ActiveSpan().(TracingSpan); ok {
			activeSpan = s
		}
	}
	return &SkyWalkingLogContext{
		TraceID:        activeSpan.GetTraceID(),
		TraceSegmentID: activeSpan.GetSegmentID(),
		SpanID:         activeSpan.GetSpanID(),
	}
}

func GetLogContextString() string {
	return GetLogContext().String()
}

func (context *SkyWalkingLogContext) String() string {
	return fmt.Sprintf("[%s,%s,%d]", context.TraceID, context.TraceSegmentID, context.SpanID)
}
