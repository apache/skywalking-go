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

package tracing

import (
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"

	"github.com/stretchr/testify/assert"

	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
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
	scx := core.SpanContext{
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
	operations    []func(existingSpans []core.Span) (core.Span, error)
	exceptedSpans []struct {
		spanType         core.SpanType
		operationName    string
		parentSpanOpName string
		peer             string
	}
}

func TestCreateSpanInSingleGoroutine(t *testing.T) {
	defer core.ResetTracingContext()
	validateSpanOperation(t, []spanOperationTestCase{
		{
			operations: []func(existingSpans []core.Span) (core.Span, error){
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
				},
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateLocalSpan("/local1")
				},
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateLocalSpan("/local1-1")
				},
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateExitSpan("/local1-1-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []core.Span) (core.Span, error) { existingSpans[3].End(); return nil, nil },
				func(existingSpans []core.Span) (core.Span, error) { existingSpans[2].End(); return nil, nil },
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateExitSpan("/local1-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []core.Span) (core.Span, error) { existingSpans[4].End(); return nil, nil },
				func(existingSpans []core.Span) (core.Span, error) { existingSpans[1].End(); return nil, nil },
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateExitSpan("/entry-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []core.Span) (core.Span, error) { existingSpans[5].End(); return nil, nil },
				func(existingSpans []core.Span) (core.Span, error) { existingSpans[0].End(); return nil, nil },
			},
			exceptedSpans: []struct {
				spanType         core.SpanType
				operationName    string
				parentSpanOpName string
				peer             string
			}{
				{core.SpanTypeEntry, "/entry", "", ""},
				{core.SpanTypeLocal, "/local1", "/entry", ""},
				{core.SpanTypeLocal, "/local1-1", "/local1", ""},
				{core.SpanTypeExit, "/local1-1-exit", "/local1-1", "localhost:8080"},
				{core.SpanTypeExit, "/local1-exit", "/local1", "localhost:8080"},
				{core.SpanTypeExit, "/entry-exit", "/entry", "localhost:8080"},
			},
		},
	})
}

func TestCreateSpanInDifferenceGoroutine(t *testing.T) {
	defer core.ResetTracingContext()
	validateSpanOperation(t, []spanOperationTestCase{
		{
			operations: []func(existingSpans []core.Span) (core.Span, error){
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
				},
				func(existingSpans []core.Span) (core.Span, error) { // new goroutine
					core.SetGLS(core.TaskTracingContextSnapshot(core.GetGLS()))
					return nil, nil
				},
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateLocalSpan("/local")
				},
				func(existingSpans []core.Span) (core.Span, error) { // new goroutine
					core.SetGLS(core.TaskTracingContextSnapshot(core.GetGLS()))
					return nil, nil
				},
				func(existingSpans []core.Span) (core.Span, error) {
					return CreateExitSpan("/local-exit", "localhost:8080", func(key, value string) error { return nil })
				},
				func(existingSpans []core.Span) (core.Span, error) {
					existingSpans[2].End()
					return nil, nil
				},
				func(existingSpans []core.Span) (core.Span, error) {
					existingSpans[1].End()
					return nil, nil
				},
				func(existingSpans []core.Span) (core.Span, error) {
					existingSpans[0].End()
					return nil, nil
				},
			},
			exceptedSpans: []struct {
				spanType         core.SpanType
				operationName    string
				parentSpanOpName string
				peer             string
			}{
				{core.SpanTypeEntry, "/entry", "", ""},
				{core.SpanTypeLocal, "/local", "/entry", ""},
				{core.SpanTypeExit, "/local-exit", "/local", "localhost:8080"},
			},
		},
	})
}

func TestSpanContextWriting(t *testing.T) {
	defer core.ResetTracingContext()
	s, err := CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
	assert.NoError(t, err)
	s.End()
	s, err = CreateExitSpan("/exit", "localhost:8080", func(key, value string) error {
		ctx := core.SpanContext{}
		if key == core.Header {
			assert.NoError(t, ctx.DecodeSW8(value))
		}
		if key == core.HeaderCorrelation {
			assert.NoError(t, ctx.DecodeSW8Correlation(value))
		}
		return nil
	})
	assert.NoError(t, err)
	s.End()
}

func TestSpanContextReading(t *testing.T) {
	defer core.ResetTracingContext()
	s, err := CreateEntrySpan("/entry", func(key string) (string, error) {
		if key == core.Header {
			return header, nil
		}
		return "", nil
	})
	assert.NoError(t, err)
	s.End()
	time.Sleep(time.Millisecond * 50)
	spans := core.GetReportedSpans()
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

func TestSpanOperation(t *testing.T) {
	defer core.ResetTracingContext()
	spanCreations := []func(op SpanOption) (core.Span, error){
		func(op SpanOption) (core.Span, error) {
			return CreateEntrySpan("test", func(headerKey string) (string, error) {
				return "", nil
			}, op)
		},
		func(op SpanOption) (core.Span, error) {
			return CreateLocalSpan("test", op)
		},
		func(op SpanOption) (core.Span, error) {
			return CreateExitSpan("test", "localhost:8080", func(headerKey, headerValue string) error {
				return nil
			}, op)
		},
	}

	spanOptions := []struct {
		op       SpanOption
		validate func(s *core.RootSegmentSpan) bool
	}{
		{WithLayer(agentv3.SpanLayer_Http), func(s *core.RootSegmentSpan) bool {
			return s.DefaultSpan.Layer == agentv3.SpanLayer_Http
		}},
		{WithComponent(1), func(s *core.RootSegmentSpan) bool {
			return s.DefaultSpan.ComponentID == 1
		}},
		{WithTag("test", "test1"), func(s *core.RootSegmentSpan) bool {
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
			span := create.(*core.RootSegmentSpan)
			assert.Truef(t, op.validate(span), "span validation failed, create index: %d, option index: %d", createInx, op)
			create.End()
		}
	}
}

func TestActiveSpan(t *testing.T) {
	defer core.ResetTracingContext()
	// active span in same goroutine
	span, err := CreateEntrySpan("/entry", func(key string) (string, error) { return "", nil })
	assert.NoError(t, err)
	assert.Equal(t, span, ActiveSpan(), "active span not correct")
	oldGLS := core.GetGLS()
	// change goroutine
	core.SetGLS(core.TaskTracingContextSnapshot(oldGLS))
	assert.Nil(t, ActiveSpan(), "active span should be nil when cross goroutine")
	// change back
	core.SetGLS(oldGLS)
	span.End()
	assert.Nil(t, ActiveSpan(), "active span not correct")
}

func TestRuntimeContext(t *testing.T) {
	defer core.ResetTracingContext()
	assert.Nilf(t, GetRuntimeContextValue("test"), "runtime context data should be nil")
	SetRuntimeContextValue("test", "test")
	assert.Equal(t, "test", GetRuntimeContextValue("test"), "runtime context data should be \"test\"")
	// switch to the new goroutine
	oldGLS := core.GetGLS()
	core.SetGLS(core.TaskTracingContextSnapshot(oldGLS))
	assert.Equal(t, "test", GetRuntimeContextValue("test"), "runtime context data should be \"test\"")
	assert.Nilf(t, GetRuntimeContextValue("test1"), "runtime context data should be nil")
	SetRuntimeContextValue("test1", "test1")
	assert.Equal(t, "test1", GetRuntimeContextValue("test1"), "runtime context data should be \"test1\"")
	// switch back to the old goroutine
	core.SetGLS(oldGLS)
	assert.Nilf(t, GetRuntimeContextValue("test1"), "runtime context data should be nil")
}

func validateSpanOperation(t *testing.T, cases []spanOperationTestCase) {
	for _, tt := range cases {
		spans := make([]core.Span, 0)
		for i, op := range tt.operations {
			span, err := op(spans)
			assert.Nilf(t, err, "create span error, operation index: %d", i)
			time.Sleep(time.Millisecond * 50)
			if span != nil {
				spans = append(spans, span)
			}
		}

		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, len(tt.exceptedSpans), len(core.GetReportedSpans()), "span count not equal")
		for i, exceptedSpan := range tt.exceptedSpans {
			var span core.DefaultSpan
			if i == 0 {
				tmp, ok := core.GetReportedSpans()[len(core.GetReportedSpans())-1-i].(*core.RootSegmentSpan)
				assert.True(t, ok, "first span is not root segment span")
				span = tmp.DefaultSpan
			} else {
				found := false
				for _, s := range core.GetReportedSpans() {
					if s.OperationName() != exceptedSpan.operationName {
						continue
					}
					tmp, ok := s.(*core.SegmentSpanImpl)
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

			assert.Greaterf(t, core.Millisecond(span.StartTime), int64(0), "start time not greater than 0, span index: %d", i)
			assert.Greaterf(t, core.Millisecond(span.EndTime), core.Millisecond(span.StartTime), "end time not greater than 0, span indeX: %d", i)
		}

		core.GetGlobalTracer().Reporter = core.NewStoreReporter()
	}
}
