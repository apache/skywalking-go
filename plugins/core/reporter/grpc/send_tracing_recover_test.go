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

package grpc

import (
	"sync/atomic"
	"testing"

	"github.com/apache/skywalking-go/plugins/core/reporter"

	commonv3 "github.com/apache/skywalking-go/protocols/collect/common/v3"
	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
)

type capturingLogger struct {
	errors int32
}

func (l *capturingLogger) WithField(key string, value interface{}) interface{} { return l }
func (l *capturingLogger) Info(args ...interface{})                            {}
func (l *capturingLogger) Infof(format string, args ...interface{})            {}
func (l *capturingLogger) Warn(args ...interface{})                            {}
func (l *capturingLogger) Warnf(format string, args ...interface{})            {}
func (l *capturingLogger) Error(args ...interface{})                           { atomic.AddInt32(&l.errors, 1) }
func (l *capturingLogger) Errorf(format string, args ...interface{})           { atomic.AddInt32(&l.errors, 1) }

// panicReportedSpan triggers a panic as soon as the transform touches it,
// simulating a corrupted span reaching SendTracing.
type panicReportedSpan struct{}

func (panicReportedSpan) Context() reporter.SegmentContext       { panic("corrupted span") }
func (panicReportedSpan) Refs() []reporter.SpanContext           { return nil }
func (panicReportedSpan) StartTime() int64                       { return 0 }
func (panicReportedSpan) EndTime() int64                         { return 0 }
func (panicReportedSpan) OperationName() string                  { return "op" }
func (panicReportedSpan) Peer() string                           { return "" }
func (panicReportedSpan) SpanType() agentv3.SpanType             { return agentv3.SpanType_Exit }
func (panicReportedSpan) SpanLayer() agentv3.SpanLayer           { return agentv3.SpanLayer_Database }
func (panicReportedSpan) IsError() bool                          { return false }
func (panicReportedSpan) Tags() []*commonv3.KeyStringValuePair   { return nil }
func (panicReportedSpan) Logs() []*agentv3.Log                   { return nil }
func (panicReportedSpan) ComponentID() int32                     { return 0 }

// TestSendTracingRecoversTransformPanic guards the recover placement in
// SendTracing: it must be registered BEFORE the transform call, because
// SendTracing runs on the segment collector goroutine which has no other
// recover - an escaping panic would kill the whole process.
func TestSendTracingRecoversTransformPanic(t *testing.T) {
	logger := &capturingLogger{}
	r := &gRPCReporter{
		logger:        logger,
		transform:     reporter.NewTransform(&reporter.Entity{ServiceName: "svc", ServiceInstanceName: "inst"}),
		tracingSendCh: make(chan *agentv3.SegmentObject, 1),
	}

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("panic escaped SendTracing (recover registered too late?): %v", p)
		}
	}()
	r.SendTracing([]reporter.ReportedSpan{panicReportedSpan{}})

	if atomic.LoadInt32(&logger.errors) == 0 {
		t.Fatal("recovered transform panic was not logged")
	}
}
