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
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
)

var tlsData interface{}
var Tracing *Tracer

func init() {
	SetGLS = func(i interface{}) {
		tlsData = i
	}
	GetGLS = func() interface{} {
		return tlsData
	}
	operator.GetOperator = func() operator.Operator {
		return Tracing
	}
	ResetTracingContext()
}

func ResetTracingContext() {
	SetGLS(nil)
	Tracing = &Tracer{initFlag: 1, Sampler: NewConstSampler(true), Reporter: &StoreReporter{}}
	SetAsNewGoroutine()
}

func SetAsNewGoroutine() {
	gls := GetGLS()
	if gls == nil {
		return
	}
	if e := gls.(ContextSnapshoter); e != nil {
		SetGLS(e.TakeSnapShot(GetGLS()))
	}
}

func GetReportedSpans() []reporter.ReportedSpan {
	return Tracing.Reporter.(*StoreReporter).Spans
}

type StoreReporter struct {
	Spans []reporter.ReportedSpan
}

func NewStoreReporter() *StoreReporter {
	return &StoreReporter{}
}

func (r *StoreReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
}

func (r *StoreReporter) Send(spans []reporter.ReportedSpan) {
	r.Spans = append(r.Spans, spans...)
}

func (r *StoreReporter) Close() {
}
