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

package operator

var GetOperator = func() Operator { return nil }
var AppendInitNotify = func(func()) {}
var MetricsAppender = func(interface{}) {}
var MetricsCollectAppender = func(func()) {}

type Operator interface {
	Tracing() interface{}     // to TracingOperator
	Logger() interface{}      // to LogOperator
	Profiler() interface{}    // to ProfileOperator
	Tools() interface{}       // to ToolsOperator
	DebugStack() []byte       // Getting the stack of the current goroutine, for getting details when plugin broken.
	Entity() interface{}      // Get the entity of the service
	Metrics() interface{}     // to MetricsOperator
	LogReporter() interface{} // to LogReporter
	So11y() interface{}       // to So11yOperator
}

type Entity interface {
	GetServiceName() string
	GetInstanceName() string
}

// OperateError reduce the "fmt" package import
type OperateError struct {
	message string
}

func (e OperateError) Error() string {
	return e.message
}

func NewError(message string) error {
	return OperateError{message: message}
}
