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
	"sync"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"

	logv3 "github.com/apache/skywalking-go/protocols/collect/logging/v3"
)

var tlsData interface{}
var Tracing *Tracer
var ReportConnectionStatus = reporter.ConnectionStatusConnected

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
	Tracing = &Tracer{initFlag: 1, Sampler: NewConstSampler(true), Reporter: &StoreReporter{},
		ServiceEntity: NewEntity("test", "test-instance"), meterMap: &sync.Map{},
		// production Boot always sets the correlation config; the tests must
		// too, otherwise correlation APIs nil-dereference (found by the
		// hostile-workload e2e). Values mirror the agent defaults.
		correlation: &CorrelationConfig{MaxKeyCount: 3, MaxValueSize: 128}}
	// Initialize ProfileManager to avoid nil pointer dereference
	Tracing.ProfileManager = NewProfileManager(nil)
	Tracing.Reporter.AddProfileTaskManager(Tracing.ProfileManager)
	SetAsNewGoroutine()
	ReportConnectionStatus = reporter.ConnectionStatusConnected
}

func SetAsNewGoroutine() {
	gls := GetGLS()
	if gls == nil {
		return
	}
	if e := gls.(ContextSnapshoter); e != nil {
		SetGLS(e.TakeSnapShot())
	}
}

func GetReportedSpans() []reporter.ReportedSpan {
	sr := Tracing.Reporter.(*StoreReporter)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return append([]reporter.ReportedSpan(nil), sr.Spans...)
}

// StoreReporter is the in-memory test reporter. SendTracing is invoked from
// the per-segment collector goroutines while tests read the results, so the
// storage must be synchronized (this used to be the test-harness data race
// that kept the full suite from running under -race).
type StoreReporter struct {
	mu      sync.Mutex
	Spans   []reporter.ReportedSpan
	Metrics []reporter.ReportedMeter
	Logs    []*logv3.LogData
}

func NewStoreReporter() *StoreReporter {
	return &StoreReporter{}
}

func (r *StoreReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
}

func (r *StoreReporter) SendTracing(spans []reporter.ReportedSpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Spans = append(r.Spans, spans...)
}

func (r *StoreReporter) SendMetrics(metrics []reporter.ReportedMeter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Metrics = append(r.Metrics, metrics...)
}

func (r *StoreReporter) SendLog(log *logv3.LogData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Logs = append(r.Logs, log)
}

func (r *StoreReporter) ConnectionStatus() reporter.ConnectionStatus {
	return ReportConnectionStatus
}

func (r *StoreReporter) Close() {
}

func (r *StoreReporter) AddProfileTaskManager(p reporter.ProfileTaskManager) {}
