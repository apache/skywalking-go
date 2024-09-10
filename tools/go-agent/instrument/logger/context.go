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
	"fmt"
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

type LogReporter interface {
	ReportLog(ctx, time interface{}, level, msg string, labels map[string]string)
	GetLogContext(withEndpoint bool) interface{}
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

func GetLogContext(withEndpoint bool) interface{} {
	report, ok := GetOperator().LogReporter().(LogReporter)
	if !ok || report == nil {
		return nil
	}

	return report.GetLogContext(withEndpoint)
}

func GetLogContextString() string {
	stringer, ok := GetLogContext(false).(fmt.Stringer)
	if !ok {
		return ""
	}

	return stringer.String()
}
