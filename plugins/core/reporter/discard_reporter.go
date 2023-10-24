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

import logv3 "skywalking.apache.org/repo/goapi/collect/logging/v3"

type discardReporter struct{}

func NewDiscardReporter() Reporter {
	return &discardReporter{}
}

func (r *discardReporter) Boot(entity *Entity, cdsWatchers []AgentConfigChangeWatcher) {
	// do nothing
}
func (r *discardReporter) SendTracing(spans []ReportedSpan) {
	// do nothing
}
func (r *discardReporter) SendMetrics(metrics []ReportedMeter) {
	// do nothing
}
func (r *discardReporter) SendLog(log *logv3.LogData) {
	// do nothing
}
func (r *discardReporter) ConnectionStatus() ConnectionStatus {
	// do nothing
	return 0
}
func (r *discardReporter) Close() {
	// do nothing
}
