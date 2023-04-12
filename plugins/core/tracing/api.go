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
	"fmt"
	"reflect"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/reporter"

	"github.com/pkg/errors"

	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

var (
	errParameter = fmt.Errorf("parameter are nil")
)

// Extractor is a tool specification which define how to
// extract trace parent context from propagation context
type Extractor func(headerKey string) (string, error)

// Injector is a tool specification which define how to
// inject trace context into propagation context
type Injector func(headerKey, headerValue string) error

func CreateEntrySpan(operationName string, extractor Extractor, opts ...SpanOption) (s core.Span, err error) {
	if operationName == "" || extractor == nil {
		return nil, errParameter
	}
	tracer, ctx, s, noop := createNoop()
	if noop {
		return s, nil
	}
	defer func() {
		saveSpanToActiveIfNotError(ctx, s, err)
	}()
	var ref = &core.SpanContext{}
	if err := ref.Decode(extractor); err != nil {
		return nil, err
	}
	if !ref.Valid {
		ref = nil
	}

	return createSpan0(tracer, s, append(opts, withRef(ref), withSpanType(core.SpanTypeEntry), withOperationName(operationName))...)
}

func CreateLocalSpan(operationName string, opts ...SpanOption) (s core.Span, err error) {
	if operationName == "" {
		return nil, errParameter
	}
	tracer, ctx, s, noop := createNoop()
	if noop {
		return s, nil
	}
	defer func() {
		saveSpanToActiveIfNotError(ctx, s, err)
	}()

	return createSpan0(tracer, s, append(opts, withSpanType(core.SpanTypeLocal), withOperationName(operationName))...)
}

func CreateExitSpan(operationName, peer string, injector Injector, opts ...SpanOption) (s core.Span, err error) {
	if operationName == "" || peer == "" || injector == nil {
		return nil, errParameter
	}
	tracer, ctx, s, noop := createNoop()
	if noop {
		return s, nil
	}
	defer func() {
		saveSpanToActiveIfNotError(ctx, s, err)
	}()

	span, err := createSpan0(tracer, s, append(opts, withSpanType(core.SpanTypeExit), withOperationName(operationName), withPeer(peer))...)
	if err != nil {
		return nil, err
	}
	spanContext := &core.SpanContext{}
	reportedSpan, ok := span.(core.SegmentSpan)
	if !ok {
		return nil, errors.New("span type is wrong")
	}

	firstSpan := reportedSpan.GetSegmentContext().FirstSpan
	spanContext.Sample = 1
	spanContext.TraceID = reportedSpan.GetSegmentContext().TraceID
	spanContext.ParentSegmentID = reportedSpan.GetSegmentContext().SegmentID
	spanContext.ParentSpanID = reportedSpan.GetSegmentContext().SpanID
	spanContext.ParentService = tracer.Service
	spanContext.ParentServiceInstance = tracer.Instance
	spanContext.ParentEndpoint = firstSpan.GetOperationName()
	spanContext.AddressUsedAtClient = peer
	spanContext.CorrelationContext = reportedSpan.GetSegmentContext().CorrelationContext

	err = spanContext.Encode(injector)
	if err != nil {
		return nil, err
	}
	return span, nil
}

func ActiveSpan() core.Span {
	ctx := getTracingContext()
	if ctx == nil || ctx.ActiveSpan == nil {
		return nil
	}
	if _, ok := ctx.ActiveSpan.(*core.SnapshotSpan); ok {
		return nil
	}
	return ctx.ActiveSpan
}

// SpanOption allows for functional options to adjust behavior
// of a Span to be created by CreateLocalSpan
type SpanOption func(s *core.DefaultSpan)

func WithLayer(layer agentv3.SpanLayer) SpanOption {
	return func(s *core.DefaultSpan) {
		s.Layer = layer
	}
}

func WithComponent(componentID int32) SpanOption {
	return func(s *core.DefaultSpan) {
		s.ComponentID = componentID
	}
}

func WithTag(key, value string) SpanOption {
	return func(s *core.DefaultSpan) {
		s.Tags = append(s.Tags, &commonv3.KeyStringValuePair{Key: key, Value: value})
	}
}

func GetRuntimeContextValue(key string) interface{} {
	context := core.GetTracingContext()
	if context == nil {
		return nil
	}
	return context.Runtime.Get(key)
}

func SetRuntimeContextValue(key string, val interface{}) {
	context := core.GetTracingContext()
	if context == nil {
		context = core.NewTracingContext()
		core.SetGLS(context)
	}
	context.Runtime.Set(key, val)
}

func createNoop() (*core.Tracer, *core.TracingContext, core.Span, bool) {
	tracer := core.GetGlobalTracer()
	if tracer == nil || !tracer.InitSuccess() {
		return nil, nil, &core.NoopSpan{}, true
	}
	ctx := getTracingContext()
	if ctx != nil {
		_, ok := ctx.ActiveSpan.(*core.NoopSpan)
		return tracer, ctx, ctx.ActiveSpan, ok
	}
	return tracer, nil, nil, false
}

func createSpan0(tracer *core.Tracer, parent core.Span, opts ...SpanOption) (s core.Span, err error) {
	ds := core.NewDefaultSpan(tracer, parent)
	for _, opt := range opts {
		opt(ds)
	}
	var parentSpan core.SegmentSpan
	if parent != nil {
		tmpSpan, ok := parent.(core.SegmentSpan)
		if ok {
			parentSpan = tmpSpan
		}
	}
	isForceSample := len(ds.Refs) > 0
	// Try to sample when it is not force sample
	if parentSpan == nil && !isForceSample {
		// Force sample
		sampled := tracer.Sampler.IsSampled(ds.OperationName)
		if !sampled {
			// Filter by sample just return noop span
			s = &core.NoopSpan{}
			return s, nil
		}
	}
	s, err = core.NewSegmentSpan(ds, parentSpan)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func withSpanType(spanType core.SpanType) SpanOption {
	return func(span *core.DefaultSpan) {
		span.SpanType = spanType
	}
}

func withOperationName(opName string) SpanOption {
	return func(span *core.DefaultSpan) {
		span.OperationName = opName
	}
}

func withRef(sc reporter.SpanContext) SpanOption {
	return func(span *core.DefaultSpan) {
		if sc == nil {
			return
		}
		v := reflect.ValueOf(sc)
		if v.Interface() == reflect.Zero(v.Type()).Interface() {
			return
		}
		span.Refs = append(span.Refs, sc)
	}
}

func withPeer(peer string) SpanOption {
	return func(span *core.DefaultSpan) {
		span.Peer = peer
	}
}

func getTracingContext() *core.TracingContext {
	ctx := core.GetGLS()
	if ctx == nil {
		return nil
	}
	return ctx.(*core.TracingContext)
}

func saveSpanToActiveIfNotError(ctx *core.TracingContext, span core.Span, err error) {
	if err != nil {
		return
	}
	if ctx == nil {
		ctx = core.NewTracingContext()
	}
	ctx.ActiveSpan = span
	core.SetGLS(ctx)
}
