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
	"fmt"
	"reflect"
	"runtime/debug"

	"github.com/pkg/errors"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

var snapshotType = reflect.TypeOf(&SnapshotSpan{})

func (t *Tracer) Tracing() interface{} {
	return t
}

func (t *Tracer) Logger() interface{} {
	return t.Log
}

func (t *Tracer) Profiler() interface{} {
	return t.ProfileManager
}

func (t *Tracer) DebugStack() []byte {
	return debug.Stack()
}

func (t *Tracer) CreateEntrySpan(operationName string, extractor interface{}, opts ...interface{}) (s interface{}, err error) {
	ctx, tracingSpan, noop := t.createNoop(operationName)
	if noop {
		return tracingSpan, nil
	}
	defer func() {
		saveSpanToActiveIfNotError(ctx, s, err)
	}()
	// if parent span is entry span, then use parent span as result
	if tracingSpan != nil && tracingSpan.IsEntry() && reflect.ValueOf(tracingSpan).Type() != snapshotType {
		tracingSpan.SetOperationName(operationName)
		// the caller becomes one more owner of the reused span and will call
		// End on it: count the reuse so only the last End freezes the span
		if segmentSpan, ok := tracingSpan.(SegmentSpan); ok {
			segmentSpan.GetDefaultSpan().enterReuse()
		}
		return tracingSpan, nil
	}
	var ref = &SpanContext{}
	if err1 := ref.Decode(extractor.(tracing.ExtractorWrapper).Fun()); err1 != nil {
		return nil, err1
	}
	if !ref.Valid {
		ref = nil
	}

	span, _, err := t.createSpan0(ctx, tracingSpan, opts, withRef(ref), withSpanType(SpanTypeEntry), withOperationName(operationName))
	if err == nil {
		sid := span.GetSegmentID()
		tid := span.GetTraceID()
		// check if is profile target
		if t.ProfileManager.CheckIfProfileTarget(operationName) {
			// check if is profiling
			if t.ProfileManager.IfProfiling() {
				if segmentSpan, ok := span.(SegmentSpan); ok {
					c := segmentSpan.GetSegmentContext()
					t.ProfileManager.TryToAddSegmentLabelSet(sid)
					t.ProfileManager.AddSpanID(tid, sid, c.SpanID)
				}
			}
		}
	}
	return span, err
}

func (t *Tracer) CreateLocalSpan(operationName string, opts ...interface{}) (s interface{}, err error) {
	ctx, tracingSpan, noop := t.createNoop(operationName)
	if noop {
		return tracingSpan, nil
	}
	defer func() {
		saveSpanToActiveIfNotError(ctx, s, err)
	}()

	span, _, err := t.createSpan0(ctx, tracingSpan, opts, withSpanType(SpanTypeLocal), withOperationName(operationName))
	if err == nil {
		sid := span.GetSegmentID()
		tid := span.GetTraceID()
		endpoint := span.GetOperationName()
		if t.ProfileManager.CheckIfProfileTarget(endpoint) {
			if segmentSpan, ok := span.(SegmentSpan); ok {
				c := segmentSpan.GetSegmentContext()
				if t.ProfileManager.IfProfiling() {
					t.ProfileManager.AddSpanID(tid, sid, c.SpanID)
				}
			}
		}
	}
	return span, err
}

func (t *Tracer) CreateExitSpan(operationName, peer string, injector interface{}, opts ...interface{}) (s interface{}, err error) {
	ctx, tracingSpan, noop := t.createNoop(operationName)
	if noop {
		return tracingSpan, nil
	}
	defer func() {
		saveSpanToActiveIfNotError(ctx, s, err)
	}()

	// if parent span is exit span, then use parent span as result
	if tracingSpan != nil && tracingSpan.IsExit() && reflect.ValueOf(tracingSpan).Type() != snapshotType {
		// the caller becomes one more owner of the reused span and will call
		// End on it: count the reuse so only the last End freezes the span
		if segmentSpan, ok := tracingSpan.(SegmentSpan); ok {
			segmentSpan.GetDefaultSpan().enterReuse()
		}
		return tracingSpan, nil
	}
	span, noop, err := t.createSpan0(ctx, tracingSpan, opts, withSpanType(SpanTypeExit), withOperationName(operationName), withPeer(peer))
	if err != nil {
		return nil, err
	}
	if noop {
		return span, nil
	}
	spanContext := &SpanContext{}
	reportedSpan, ok := span.(SegmentSpan)
	if !ok {
		return nil, errors.New(fmt.Sprintf("span type is wrong: %T", span))
	}

	firstSpan := reportedSpan.GetSegmentContext().FirstSpan
	spanContext.Sample = 1
	spanContext.TraceID = reportedSpan.GetSegmentContext().TraceID
	spanContext.ParentSegmentID = reportedSpan.GetSegmentContext().SegmentID
	spanContext.ParentSpanID = reportedSpan.GetSegmentContext().SpanID
	spanContext.ParentService = t.ServiceEntity.ServiceName
	spanContext.ParentServiceInstance = t.ServiceEntity.ServiceInstanceName
	spanContext.ParentEndpoint = firstSpan.GetOperationName()
	spanContext.AddressUsedAtClient = peer
	// Snapshot, not the live map: the propagation header encoding iterates this
	// map while other goroutines of the segment may concurrently set correlation
	// values, which would be a fatal concurrent map iteration and map write.
	spanContext.CorrelationContext = reportedSpan.GetSegmentContext().CorrelationContext.Snapshot()

	err = spanContext.Encode(injector.(tracing.InjectorWrapper).Fun())
	if err != nil {
		return nil, err
	}
	return span, nil
}

// ExtractContext decodes the propagated context carried by extractor and
// attaches it to the current active entry span as one more segment reference,
// merging the carried correlation values - the equivalent of the Java agent's
// ContextManager.extract, used by batch consumers to link every upstream
// message to the single entry span.
func (t *Tracer) ExtractContext(extractor interface{}) error {
	ctx := getTracingContext()
	if ctx == nil || ctx.ActiveSpan() == nil {
		return nil
	}
	segmentSpan, ok := ctx.ActiveSpan().(SegmentSpan)
	if !ok || !segmentSpan.GetDefaultSpan().IsEntry() {
		// only an entry span carries upstream references, mirroring the Java agent
		return nil
	}
	ref := &SpanContext{}
	if err := ref.Decode(extractor.(tracing.ExtractorWrapper).Fun()); err != nil {
		return err
	}
	if !ref.Valid {
		return nil
	}
	segmentSpan.GetDefaultSpan().appendRef(ref)
	// merge the carried correlation into the segment, last write wins
	// (mirroring the Java agent's extractCorrelationTo)
	correlation := segmentSpan.GetSegmentContext().CorrelationContext
	for k, v := range ref.CorrelationContext {
		correlation.Set(k, v)
	}
	return nil
}

func (t *Tracer) ActiveSpan() interface{} {
	ctx := getTracingContext()
	if ctx == nil || ctx.ActiveSpan() == nil {
		return nil
	}
	span := ctx.ActiveSpan()
	return span
}

func (t *Tracer) GetRuntimeContextValue(key string) interface{} {
	context := getTracingContext()
	if context == nil {
		return nil
	}
	return context.Runtime.Get(key)
}

func (t *Tracer) SetRuntimeContextValue(key string, value interface{}) {
	context := getTracingContext()
	if context == nil {
		context = NewTracingContext()
		SetGLS(context)
	}
	context.Runtime.Set(key, value)
}

func (t *Tracer) CaptureContext() interface{} {
	ctx := getTracingContext()
	if ctx == nil {
		return nil
	}
	snapshot := &ContextSnapshot{
		activeSpan: newSnapshotSpan(ctx.ActiveSpan()),
		runtime:    ctx.Runtime.clone(),
	}
	return snapshot
}

func (t *Tracer) ContinueContext(snapshot interface{}) {
	if snapshot == nil {
		return
	}
	if snap, ok := snapshot.(*ContextSnapshot); ok {
		ctx := getTracingContext()
		if ctx == nil {
			ctx = NewTracingContext()
			SetGLS(ctx)
		}
		ctx.activeSpanLock.Lock()
		defer ctx.activeSpanLock.Unlock()
		ctx.activeSpan = snap.activeSpan
		// Clone on continue as well (capture already clones): the same snapshot
		// may be continued by multiple goroutines (e.g. the send and receive
		// goroutines of one gRPC stream), and sharing one RuntimeContext map
		// between them would be a fatal concurrent map read/write.
		ctx.Runtime = snap.runtime.clone()
	}
}

func (t *Tracer) CleanContext() {
	SetGLS(nil)
}

func (t *Tracer) GetCorrelationContextValue(key string) string {
	span := t.ActiveSpan()
	if span == nil {
		return ""
	}
	switch span.(type) {
	case *SegmentSpanImpl, *RootSegmentSpan:
		segCtx := span.(SegmentSpan).GetSegmentContext()
		return segCtx.GetCorrelationContextValue(key)
	default:
		return ""
	}
}

func (t *Tracer) SetCorrelationContextValue(key, value string) {
	span := t.ActiveSpan()
	if span == nil {
		return
	}
	switch span.(type) {
	case *SegmentSpanImpl, *RootSegmentSpan:
		if len(value) > t.correlation.MaxValueSize {
			return
		}
		segCtx := span.(SegmentSpan).GetSegmentContext()
		// Len/Set are two separate lock acquisitions, so concurrent writers can
		// exceed MaxKeyCount by a few entries. The limit is a soft bound (same
		// behavior as the pre-synchronization bare map) - keeping the two calls
		// separate avoids a combined check-and-set API for a non-issue.
		if segCtx.CorrelationContext.Len() >= t.correlation.MaxKeyCount {
			return
		}
		segCtx.SetCorrelationContextValue(key, value)
	default:
	}
}

type ContextSnapshot struct {
	activeSpan TracingSpan
	// runtime is cloned at capture time and treated as IMMUTABLE afterwards:
	// ContinueContext may read it concurrently from several goroutines (it
	// clones again per continue), so never mutate it through the snapshot.
	runtime *RuntimeContext
}

func (s *ContextSnapshot) IsValid() bool {
	return s.activeSpan != nil && s.runtime != nil
}

func (t *Tracer) createNoop(operationName string) (*TracingContext, TracingSpan, bool) {
	ctx := getTracingContext()
	if ctx != nil {
		span := ctx.ActiveSpan()
		noop, ok := span.(*NoopSpan)
		if ok {
			// increase the stack count for ensure the noop span can be clear in the context
			noop.enterNoSpan()
		}
		return ctx, span, ok
	}
	if !t.InitSuccess() || t.Reporter.ConnectionStatus() == reporter.ConnectionStatusDisconnect {
		GetSo11y(t).MeasureTracingContextCreation(false, true)
		return nil, newNoopSpan(t), true
	}
	if tracerIgnore(operationName, t.ignoreSuffix, t.traceIgnorePath) {
		GetSo11y(t).MeasureTracingContextCreation(false, true)
		return nil, newNoopSpan(t), true
	}
	ctx = NewTracingContext()
	return ctx, nil, false
}

func (t *Tracer) createSpan0(ctx *TracingContext, parent TracingSpan, pluginOpts []interface{},
	coreOpts ...interface{}) (s TracingSpan, noop bool, err error) {
	ds := NewDefaultSpan(t, parent)
	var parentSpan SegmentSpan
	if parent != nil {
		tmpSpan, ok := parent.(SegmentSpan)
		if ok {
			parentSpan = tmpSpan
		}
	}
	isForceSample := len(ds.Refs) > 0
	// Try to sample when it is not force sample
	if parentSpan == nil && !isForceSample {
		isSampled := t.Sampler.IsSampled(ds.OperationName)
		if !isSampled {
			GetSo11y(t).MeasureTracingContextCreation(false, true)
			GetSo11y(t).MeasureLeakedTracingContext(true)
			// Filter by sample just return noop span
			return newNoopSpan(t), true, nil
		}
	}
	// process the opts from agent core for prepare building segment span
	for _, opt := range coreOpts {
		opt.(tracing.SpanOption).Apply(ds)
	}
	s, err = NewSegmentSpan(ctx, ds, parentSpan)
	if err != nil {
		return nil, false, err
	}
	// process the opts from plugin, split opts because the DefaultSpan not contains the tracing context information(AdaptSpan)
	for _, opt := range pluginOpts {
		opt.(tracing.SpanOption).Apply(s)
	}
	GetSo11y(t).MeasureTracingContextCreation(isForceSample, false)
	return s, false, nil
}

func withSpanType(spanType SpanType) tracing.SpanOption {
	return buildSpanOption(func(span *DefaultSpan) {
		span.SpanType = spanType
	})
}

func withOperationName(opName string) tracing.SpanOption {
	return buildSpanOption(func(span *DefaultSpan) {
		span.OperationName = opName
	})
}

func withRef(sc reporter.SpanContext) tracing.SpanOption {
	return buildSpanOption(func(span *DefaultSpan) {
		if sc == nil {
			return
		}
		v := reflect.ValueOf(sc)
		if v.Interface() == reflect.Zero(v.Type()).Interface() {
			return
		}
		span.Refs = append(span.Refs, sc)
	})
}

func withPeer(peer string) tracing.SpanOption {
	return buildSpanOption(func(span *DefaultSpan) {
		span.Peer = peer
	})
}

type spanOpImpl struct {
	exe func(s *DefaultSpan)
}

func (s *spanOpImpl) Apply(span interface{}) {
	if segmentSpan, ok := span.(*DefaultSpan); ok {
		s.exe(segmentSpan)
	}
}

func buildSpanOption(e func(s *DefaultSpan)) tracing.SpanOption {
	return &spanOpImpl{exe: e}
}

func getTracingContext() *TracingContext {
	gls := GetGLS()
	if gls == nil {
		return nil
	}
	return gls.(*TracingContext)
}

func saveSpanToActiveIfNotError(ctx *TracingContext, span interface{}, err error) {
	if err != nil || span == nil {
		return
	}
	ctx.SaveActiveSpan(span.(TracingSpan))
	SetGLS(ctx)
}
