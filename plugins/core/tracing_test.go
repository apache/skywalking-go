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
	"time"

	"github.com/stretchr/testify/assert"

	agentv3 "github.com/apache/skywalking-go/protocols/skywalking.apache.org/repo/goapi/collect/language/agent/v3"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	sample                = 1
	traceID               = "1f2d4bf47bf711eab794acde48001122"
	parentSegmentID       = "1e7c204a7bf711eab858acde48001122"
	parentSpanID          = 0
	parentService         = "service"
	parentServiceInstance = "instance"
	parentEndpoint        = "/foo/bar"
	addressUsedAtClient   = "foo.svc:8787"
)

var header string

func init() {
	scx := SpanContext{
		Sample:                sample,
		TraceID:               traceID,
		ParentSegmentID:       parentSegmentID,
		ParentSpanID:          parentSpanID,
		ParentService:         parentService,
		ParentServiceInstance: parentServiceInstance,
		ParentEndpoint:        parentEndpoint,
		AddressUsedAtClient:   addressUsedAtClient,
	}
	header = scx.EncodeSW8()
}

type spanOperationTestCase struct {
	operations    []func(existingSpans []tracing.Span) (tracing.Span, error)
	exceptedSpans []struct {
		spanType         SpanType
		operationName    string
		parentSpanOpName string
		peer             string
	}
}

func TestCreateSpanInSingleGoroutine(t *testing.T) {
	defer ResetTracingContext()
	validateSpanOperation(t, []spanOperationTestCase{
		{
			operations: []func(existingSpans []tracing.Span) (tracing.Span, error){
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateLocalSpan("/local1")
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateLocalSpan("/local1-1")
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateExitSpan("/local1-1-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[3].End()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[2].End()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateExitSpan("/local1-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[4].End()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[1].End()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateExitSpan("/entry-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[5].End()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[0].End()
					return nil, nil
				},
			},
			exceptedSpans: []struct {
				spanType         SpanType
				operationName    string
				parentSpanOpName string
				peer             string
			}{
				{SpanTypeEntry, "/entry", "", ""},
				{SpanTypeLocal, "/local1", "/entry", ""},
				{SpanTypeLocal, "/local1-1", "/local1", ""},
				{SpanTypeExit, "/local1-1-exit", "/local1-1", "localhost:8080"},
				{SpanTypeExit, "/local1-exit", "/local1", "localhost:8080"},
				{SpanTypeExit, "/entry-exit", "/entry", "localhost:8080"},
			},
		},
	})
}

func TestCreateSpanInDifferenceGoroutine(t *testing.T) {
	defer ResetTracingContext()
	validateSpanOperation(t, []spanOperationTestCase{
		{
			operations: []func(existingSpans []tracing.Span) (tracing.Span, error){
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) { // new goroutine
					SetAsNewGoroutine()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateLocalSpan("/local")
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) { // new goroutine
					SetAsNewGoroutine()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateExitSpan("/local-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[2].End()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[1].End()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					existingSpans[0].End()
					return nil, nil
				},
			},
			exceptedSpans: []struct {
				spanType         SpanType
				operationName    string
				parentSpanOpName string
				peer             string
			}{
				{SpanTypeEntry, "/entry", "", ""},
				{SpanTypeLocal, "/local", "/entry", ""},
				{SpanTypeExit, "/local-exit", "/local", "localhost:8080"},
			},
		},
	})
}

func TestOverlapSpanOperation(t *testing.T) {
	defer ResetTracingContext()
	validateSpanOperation(t, []spanOperationTestCase{
		{
			operations: []func(existingSpans []tracing.Span) (tracing.Span, error){
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateEntrySpan("/entry1", func(key string) (string, error) { return "", nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					// the operation name of exit span overlap should follow the last exit span operation name
					return tracing.CreateEntrySpan("/entry1-update", func(key string) (string, error) { return "", nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) { // new goroutine
					SetAsNewGoroutine()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateEntrySpan("/entry2", func(key string) (string, error) { return "", nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateExitSpan("/exit1", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					// the operation name of exit span overlap should follow the first exit span operation name
					return tracing.CreateExitSpan("/exit1-update", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) { // new goroutine
					SetAsNewGoroutine()
					return nil, nil
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					return tracing.CreateExitSpan("/exit2", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []tracing.Span) (tracing.Span, error) {
					for _, s := range existingSpans {
						s.End()
					}
					return nil, nil
				},
			},
			exceptedSpans: []struct {
				spanType         SpanType
				operationName    string
				parentSpanOpName string
				peer             string
			}{
				{SpanTypeEntry, "/entry1-update", "", ""},
				{SpanTypeEntry, "/entry2", "/entry1-update", ""},
				{SpanTypeExit, "/exit1", "/entry2", "localhost:8080"},
				{SpanTypeExit, "/exit2", "/exit1", "localhost:8080"},
			},
		},
	})
}

func TestSpanContextWriting(t *testing.T) {
	defer ResetTracingContext()
	s, err := tracing.CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
	assert.NoError(t, err)
	s.End()
	s, err = tracing.CreateExitSpan("/exit", "localhost:8080", func(key, value string) error {
		ctx := SpanContext{}
		if key == Header {
			assert.NoError(t, ctx.DecodeSW8(value))
		}
		if key == HeaderCorrelation {
			assert.NoError(t, ctx.DecodeSW8Correlation(value))
		}
		return nil
	})
	assert.NoError(t, err)
	s.End()
}

func TestSpanContextReading(t *testing.T) {
	defer ResetTracingContext()
	s, err := tracing.CreateEntrySpan("/entry", func(key string) (string, error) {
		if key == Header {
			return header, nil
		}
		return "", nil
	})
	assert.NoError(t, err)
	s.End()
	time.Sleep(time.Millisecond * 50)
	spans := GetReportedSpans()
	assert.Equal(t, 1, len(spans), "span count not correct")
	tracingContext := spans[0].Context()
	assert.Equal(t, traceID, tracingContext.GetTraceID(), "trace id not correct")
	assert.Equal(t, 1, len(spans[0].Refs()), "refs not correct")
	refCtx := spans[0].Refs()[0]
	assert.Equal(t, traceID, refCtx.GetTraceID(), "ref trace id not correct")
	assert.Equal(t, parentSegmentID, refCtx.GetParentSegmentID(), "ref segment id not correct")
	assert.Equal(t, parentEndpoint, refCtx.GetParentEndpoint(), "ref endpoint not correct")
	assert.Equal(t, int32(parentSpanID), refCtx.GetParentSpanID(), "ref span id not correct")
	assert.Equal(t, parentService, refCtx.GetParentService(), "ref service not correct")
	assert.Equal(t, parentServiceInstance, refCtx.GetParentServiceInstance(), "ref service instance not correct")
	assert.Equal(t, parentEndpoint, refCtx.GetParentEndpoint(), "ref endpoint not correct")
}

func TestReporterDisconnect(t *testing.T) {
	defer ResetTracingContext()
	ReportConnectionStatus = reporter.ConnectionStatusDisconnect
	s, err := tracing.CreateEntrySpan("/entry", func(key string) (string, error) {
		return "", nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, s, "span should not be nil")
	s.End()
	time.Sleep(time.Millisecond * 50)
	spans := GetReportedSpans()
	assert.Equal(t, 0, len(spans), "should no span been collected")
}

func TestSpanOperation(t *testing.T) {
	defer ResetTracingContext()
	spanCreations := []func(op tracing.SpanOption) (tracing.Span, error){
		func(op tracing.SpanOption) (tracing.Span, error) {
			return tracing.CreateEntrySpan("test", func(headerKey string) (string, error) {
				return "", nil
			}, op)
		},
		func(op tracing.SpanOption) (tracing.Span, error) {
			return tracing.CreateLocalSpan("test", op)
		},
		func(op tracing.SpanOption) (tracing.Span, error) {
			return tracing.CreateExitSpan("test", "localhost:8080", func(headerKey, headerValue string) error {
				return nil
			}, op)
		},
	}

	spanOptions := []struct {
		op       tracing.SpanOption
		validate func(s *RootSegmentSpan) bool
	}{
		{tracing.WithLayer(tracing.SpanLayerHTTP), func(s *RootSegmentSpan) bool {
			return s.DefaultSpan.Layer == agentv3.SpanLayer_Http
		}},
		{tracing.WithComponent(1), func(s *RootSegmentSpan) bool {
			return s.DefaultSpan.ComponentID == 1
		}},
		{tracing.WithTag("test", "test1"), func(s *RootSegmentSpan) bool {
			for _, k := range s.DefaultSpan.Tags {
				if k.Key == "test" {
					return k.Value == "test1"
				}
			}
			return false
		}},
	}

	for createInx, spanCreate := range spanCreations {
		for _, op := range spanOptions {
			create, err := spanCreate(op.op)
			if err != nil {
				assert.Nil(t, err, "create span error")
			}
			span := create.(*tracing.SpanWrapper).Span.(*RootSegmentSpan)
			assert.Truef(t, op.validate(span), "span validation failed, create index: %d, option index: %d", createInx, op)
			create.End()
		}
	}
}

func TestActiveSpan(t *testing.T) {
	defer ResetTracingContext()
	// active span in same goroutine
	span, err := tracing.CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
	assert.NoError(t, err)
	assert.Equal(t, span, tracing.ActiveSpan(), "active span not correct")
	oldGLS := GetGLS()
	// change goroutine
	SetAsNewGoroutine()
	assert.NotNil(t, tracing.ActiveSpan(), "active span should be nil when cross goroutine")
	// change back
	SetGLS(oldGLS)
	span.End()
	assert.Nil(t, tracing.ActiveSpan(), "active span not correct")
}

func TestRuntimeContext(t *testing.T) {
	defer ResetTracingContext()
	assert.Nilf(t, tracing.GetRuntimeContextValue("test"), "runtime context data should be nil")
	tracing.SetRuntimeContextValue("test", "test")
	assert.Equal(t, "test", tracing.GetRuntimeContextValue("test"), "runtime context data should be \"test\"")
	// switch to the new goroutine
	oldGLS := GetGLS()
	SetAsNewGoroutine()
	assert.Equal(t, "test", tracing.GetRuntimeContextValue("test"), "runtime context data should be \"test\"")
	assert.Nilf(t, tracing.GetRuntimeContextValue("test1"), "runtime context data should be nil")
	tracing.SetRuntimeContextValue("test1", "test1")
	assert.Equal(t, "test1", tracing.GetRuntimeContextValue("test1"), "runtime context data should be \"test1\"")
	// switch back to the old goroutine
	SetGLS(oldGLS)
	assert.Nilf(t, tracing.GetRuntimeContextValue("test1"), "runtime context data should be nil")
}

func TestAsyncSpan(t *testing.T) {
	defer ResetTracingContext()
	span, err := tracing.CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
	assert.Nil(t, err, "create span error")
	assert.NotNil(t, span, "span should not be nil")
	span.PrepareAsync()
	span.End()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, len(GetReportedSpans()), "span should not be reported")

	// add times for async finish(make sure the end time of span should be updated)
	time.Sleep(100 * time.Millisecond)
	span.AsyncFinish()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, len(GetReportedSpans()), "span should not be reported")
	s := GetReportedSpans()[0]
	assert.Equal(t, "/entry", s.OperationName(), "span operation name should be \"/entry\"")
	assert.Equal(t, agentv3.SpanType_Entry, s.SpanType(), "span type should be entry")
	assert.Equal(t, 0, len(s.Refs()), "span refs should be empty")
	assert.Greater(t, s.EndTime(), s.StartTime()+150, "span end time should be greater than start time + 150")
}

func TestContext(t *testing.T) {
	defer ResetTracingContext()
	span, err := tracing.CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
	assert.Nil(t, err, "create span error")
	assert.NotNil(t, span, "span should not be nil")

	snapshot := tracing.CaptureContext()
	assert.NotNil(t, snapshot, "snapshot should not be nil")
	assert.True(t, snapshot.IsValid(), "snapshot should be valid")

	SetAsNewGoroutine()
	tracing.ContinueContext(snapshot)
	activeSpan := tracing.ActiveSpan()
	assert.NotNil(t, activeSpan, "active span should not be nil")

	tracing.CleanContext()
	assert.Nil(t, tracing.ActiveSpan(), "active span should be nil")
}

func TestNoopSpan(t *testing.T) {
	defer ResetTracingContext()
	Tracing.Sampler = NewConstSampler(false)
	var err error
	// create multiple noop span
	span, err := tracing.CreateLocalSpan("test")
	assert.Nil(t, err, "create span error")
	assert.NotNil(t, span, "span should not be nil")
	span1, err := tracing.CreateLocalSpan("test2")
	assert.Nil(t, err, "create span error")
	assert.NotNil(t, span, "span should not be nil")
	activeSpan := tracing.ActiveSpan()
	assert.NotNil(t, activeSpan, "active span should not be nil")
	context := tracing.CaptureContext()
	assert.NotNil(t, context, "context should not be nil")
	oldGLS := GetGLS()

	// switch to a new GLS(continue context and create span test)
	SetAsNewGoroutine()
	tracing.ContinueContext(context)
	assert.NotNil(t, tracing.ActiveSpan(), "active span should not be nil")
	test3span, err := tracing.CreateLocalSpan("test3")
	assert.Nil(t, err, "create span error")
	assert.NotNil(t, test3span, "span should not be nil")
	test3span.End()
	activeSpan = tracing.ActiveSpan()
	assert.Nil(t, activeSpan, "active span should be nil")

	// switch back to the GLS(check the active span should be nil, make sure context is clean)
	SetGLS(oldGLS)
	span1.End()
	span.End()
	activeSpan = tracing.ActiveSpan()
	assert.Nil(t, activeSpan, "active span should be nil")
}

func validateSpanOperation(t *testing.T, cases []spanOperationTestCase) {
	for _, tt := range cases {
		spans := make([]tracing.Span, 0)
		for i, op := range tt.operations {
			span, err := op(spans)
			assert.Nilf(t, err, "create span error, operation index: %d", i)
			time.Sleep(time.Millisecond * 50)
			if span != nil {
				spans = append(spans, span)
			}
		}

		time.Sleep(time.Millisecond * 100)
		reportedSpans := GetReportedSpans()
		assert.Equal(t, len(tt.exceptedSpans), len(reportedSpans), "span count not equal")
		for i, exceptedSpan := range tt.exceptedSpans {
			var span DefaultSpan
			if i == 0 {
				tmp, ok := reportedSpans[len(reportedSpans)-1-i].(*RootSegmentSpan)
				assert.True(t, ok, "first span is not root segment span")
				span = tmp.DefaultSpan
			} else {
				found := false
				for _, s := range reportedSpans {
					if s.OperationName() != exceptedSpan.operationName {
						continue
					}
					tmp, ok := s.(*SegmentSpanImpl)
					assert.Truef(t, ok, "span is not segment span, span index: %d", i)
					span = tmp.DefaultSpan
					found = true
					break
				}
				assert.Truef(t, found, "span not found, span index: %d, name: %s", i, exceptedSpan.operationName)
			}
			assert.Equalf(t, exceptedSpan.spanType, span.SpanType, "span type not equal, span index: %d", i)
			assert.Equalf(t, exceptedSpan.operationName, span.OperationName, "operation name not equal, span index: %d", i)
			if exceptedSpan.parentSpanOpName != "" {
				assert.Equalf(t, exceptedSpan.parentSpanOpName, span.Parent.GetOperationName(), "parent operation name not equal, span index: %d", i)
			} else {
				assert.Nilf(t, span.Parent, "parent span not nil, span index: %d", i)
			}
			if exceptedSpan.peer != "" {
				assert.Equalf(t, exceptedSpan.peer, span.Peer, "span peer not equal, span index: %d", i)
			} else {
				assert.Truef(t, span.Peer == "", "span peer not empty, span index: %d", i)
			}

			assert.Greaterf(t, Millisecond(span.StartTime), int64(0), "start time not greater than 0, span index: %d", i)
			assert.Greaterf(t, Millisecond(span.EndTime), Millisecond(span.StartTime), "end time not greater than 0, span indeX: %d", i)
		}

		Tracing.Reporter = NewStoreReporter()
	}
}
