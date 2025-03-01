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
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const noopContextValue = "N/A"

type NoopSpan struct {
	stackCount int
	tracer     *Tracer
}

func newSnapshotNoopSpan() *NoopSpan {
	// snapshot noop span is not a real span
	return &NoopSpan{
		stackCount: 0,
	}
}

func newNoopSpan(tracer *Tracer) *NoopSpan {
	return &NoopSpan{
		stackCount: 1,
		tracer:     tracer,
	}
}

func (*NoopSpan) GetTraceID() string {
	return noopContextValue
}

func (*NoopSpan) GetSegmentID() string {
	return noopContextValue
}

func (*NoopSpan) GetSpanID() int32 {
	return -1
}

func (*NoopSpan) SetOperationName(string) {
}

func (*NoopSpan) GetOperationName() string {
	return ""
}

func (*NoopSpan) SetPeer(string) {
}

func (*NoopSpan) GetPeer() string {
	return ""
}

func (*NoopSpan) SetSpanLayer(layer int32) {
}

func (*NoopSpan) GetSpanLayer() agentv3.SpanLayer {
	return 0
}

func (*NoopSpan) SetComponent(int32) {
}

func (*NoopSpan) GetComponent() int32 {
	return 0
}

func (*NoopSpan) Tag(string, string) {
}

func (*NoopSpan) Log(...string) {
}

func (*NoopSpan) Error(...string) {
}

func (*NoopSpan) ErrorOccured() {
}

func (n *NoopSpan) enterNoSpan() {
	n.stackCount++
}

func (n *NoopSpan) End() {
	n.stackCount--
	if n.stackCount == 0 {
		GetSo11y(n.tracer).MeasureTracingContextCompletion(true)
		if ctx := getTracingContext(); ctx != nil {
			ctx.SaveActiveSpan(nil)
		}
	}
}

func (*NoopSpan) IsEntry() bool {
	return false
}

func (*NoopSpan) IsExit() bool {
	return false
}

func (*NoopSpan) IsValid() bool {
	return true
}

func (n *NoopSpan) ParentSpan() TracingSpan {
	return nil
}

func (n *NoopSpan) PrepareAsync() {
}

func (n *NoopSpan) AsyncFinish() {
}

func (n *NoopSpan) GetEndPointName() string {
	return ""
}

func (n *NoopSpan) GetParentSpan() interface{} {
	return nil
}
