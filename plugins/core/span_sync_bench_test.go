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

	"github.com/apache/skywalking-go/plugins/core/reporter"

	logv3 "github.com/apache/skywalking-go/protocols/collect/logging/v3"
)

// benchNopReporter discards everything so the benchmark measures the span
// machinery itself rather than a reporter implementation.
type benchNopReporter struct{}

func (benchNopReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
}
func (benchNopReporter) SendTracing(spans []reporter.ReportedSpan)    {}
func (benchNopReporter) SendMetrics(metrics []reporter.ReportedMeter) {}
func (benchNopReporter) SendLog(log *logv3.LogData)                   {}
func (benchNopReporter) ConnectionStatus() reporter.ConnectionStatus {
	return reporter.ConnectionStatusConnected
}
func (benchNopReporter) Close()                                              {}
func (benchNopReporter) AddProfileTaskManager(p reporter.ProfileTaskManager) {}

// BenchmarkSpanLifecycleRealAgent measures the genuine span hot path
// (CreateLocalSpan -> rename -> tags incl. a same-key rewrite -> log -> error
// -> End, through the real sampler/segment/GLS/collector machinery) and serves
// as the performance regression guard for the per-span locking introduced for
// apache/skywalking#13885.
func BenchmarkSpanLifecycleRealAgent(b *testing.B) {
	ResetTracingContext()
	Tracing.Reporter = benchNopReporter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, err := Tracing.CreateLocalSpan("bench/op")
		if err != nil {
			b.Fatal(err)
		}
		span := s.(TracingSpan)
		span.SetOperationName("bench/op/renamed")
		span.Tag("db.type", "sql")
		span.Tag("db.instance", "benchdb")
		span.Tag("db.type", "mysql")
		span.Log("event", "cache-miss")
		span.Error("error", "boom")
		span.End()
	}
	b.StopTimer()
	// restore clean state for other tests in the package
	ResetTracingContext()
}
