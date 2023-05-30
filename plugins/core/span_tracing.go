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
	"sync/atomic"

	"github.com/apache/skywalking-go/plugins/core/reporter"

	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func NewSegmentSpan(ctx *TracingContext, defaultSpan *DefaultSpan, parentSpan SegmentSpan) (s SegmentSpan, err error) {
	ssi := &SegmentSpanImpl{
		DefaultSpan: *defaultSpan,
	}
	err = ssi.createSegmentContext(ctx, parentSpan)
	if err != nil {
		return nil, err
	}
	if parentSpan == nil || !parentSpan.segmentRegister() {
		rs := newSegmentRoot(ssi)
		err = rs.createRootSegmentContext(ctx, parentSpan)
		if err != nil {
			return nil, err
		}
		s = rs
	} else {
		s = ssi
	}
	return
}

// SegmentContext is the context in a segment
type SegmentContext struct {
	TraceID            string
	SegmentID          string
	SpanID             int32
	ParentSpanID       int32
	ParentSegmentID    string
	collect            chan<- reporter.ReportedSpan
	refNum             *int32
	spanIDGenerator    *int32
	FirstSpan          TracingSpan `json:"-"`
	CorrelationContext map[string]string
}

func (c *SegmentContext) GetTraceID() string {
	return c.TraceID
}

func (c *SegmentContext) GetSegmentID() string {
	return c.SegmentID
}

func (c *SegmentContext) GetSpanID() int32 {
	return c.SpanID
}

func (c *SegmentContext) GetParentSpanID() int32 {
	return c.ParentSpanID
}

func (c *SegmentContext) GetParentSegmentID() string {
	return c.ParentSegmentID
}

type SegmentSpan interface {
	TracingSpan
	GetSegmentContext() SegmentContext
	tracer() *Tracer
	segmentRegister() bool
	GetDefaultSpan() *DefaultSpan
}

type SegmentSpanImpl struct {
	DefaultSpan
	SegmentContext
}

// For TracingSpan
func (s *SegmentSpanImpl) End() {
	if !s.IsValid() {
		return
	}
	s.DefaultSpan.End(true)
	if !s.DefaultSpan.InAsyncMode {
		s.end0()
	}
}

func (s *SegmentSpanImpl) AsyncFinish() {
	s.DefaultSpan.AsyncFinish()
	s.DefaultSpan.End(false)
	s.end0()
}

func (s *SegmentSpanImpl) end0() {
	go func() {
		s.SegmentContext.collect <- s
	}()
}
func (s *SegmentSpanImpl) GetDefaultSpan() *DefaultSpan {
	return &s.DefaultSpan
}

// For Reported TracingSpan

func (s *SegmentSpanImpl) Context() reporter.SegmentContext {
	return &s.SegmentContext
}

func (s *SegmentSpanImpl) Refs() []reporter.SpanContext {
	return s.DefaultSpan.Refs
}

func (s *SegmentSpanImpl) StartTime() int64 {
	return Millisecond(s.DefaultSpan.StartTime)
}

func (s *SegmentSpanImpl) EndTime() int64 {
	return Millisecond(s.DefaultSpan.EndTime)
}

func (s *SegmentSpanImpl) OperationName() string {
	return s.DefaultSpan.OperationName
}

func (s *SegmentSpanImpl) Peer() string {
	return s.DefaultSpan.Peer
}

func (s *SegmentSpanImpl) SpanType() agentv3.SpanType {
	return agentv3.SpanType(s.DefaultSpan.SpanType)
}

func (s *SegmentSpanImpl) SpanLayer() agentv3.SpanLayer {
	return s.DefaultSpan.Layer
}

func (s *SegmentSpanImpl) IsError() bool {
	return s.DefaultSpan.IsError
}

func (s *SegmentSpanImpl) Tags() []*commonv3.KeyStringValuePair {
	return s.DefaultSpan.Tags
}

func (s *SegmentSpanImpl) Logs() []*agentv3.Log {
	return s.DefaultSpan.Logs
}

func (s *SegmentSpanImpl) ComponentID() int32 {
	return s.DefaultSpan.ComponentID
}

func (s *SegmentSpanImpl) GetSegmentContext() SegmentContext {
	return s.SegmentContext
}

func (s *SegmentSpanImpl) tracer() *Tracer {
	return s.DefaultSpan.tracer
}

func (s *SegmentSpanImpl) ContinueContext() {
	if !s.InAsyncMode {
		panic("not in async mode")
	}
	context := getTracingContext()
	if context == nil {
		context = NewTracingContext()
	}
	saveSpanToActiveIfNotError(context, s, nil)
}

func (s *SegmentSpanImpl) segmentRegister() bool {
	for {
		o := atomic.LoadInt32(s.SegmentContext.refNum)
		if o < 0 {
			return false
		}
		if atomic.CompareAndSwapInt32(s.SegmentContext.refNum, o, o+1) {
			return true
		}
	}
}

func (s *SegmentSpanImpl) createSegmentContext(ctx *TracingContext, parent SegmentSpan) (err error) {
	if parent == nil {
		s.SegmentContext = SegmentContext{}
		if len(s.DefaultSpan.Refs) > 0 {
			s.TraceID = s.DefaultSpan.Refs[0].GetTraceID()
			s.CorrelationContext = s.DefaultSpan.Refs[0].(*SpanContext).CorrelationContext
		} else {
			s.TraceID, err = GenerateGlobalID(ctx)
			if err != nil {
				return err
			}
			s.CorrelationContext = make(map[string]string)
		}
	} else {
		s.SegmentContext = parent.GetSegmentContext()
		s.ParentSegmentID = s.GetSegmentID()
		s.ParentSpanID = s.GetSpanID()
		s.SpanID = atomic.AddInt32(s.SegmentContext.spanIDGenerator, 1)
		s.CorrelationContext = parent.GetSegmentContext().CorrelationContext
	}
	if s.SegmentContext.FirstSpan == nil {
		s.SegmentContext.FirstSpan = s
	}
	if s.CorrelationContext == nil {
		s.CorrelationContext = make(map[string]string)
	}
	return
}

type RootSegmentSpan struct {
	*SegmentSpanImpl
	notify  <-chan reporter.ReportedSpan
	segment []reporter.ReportedSpan
	doneCh  chan int32
}

func (rs *RootSegmentSpan) End() {
	if !rs.IsValid() {
		return
	}
	rs.DefaultSpan.End(true)
	if !rs.InAsyncMode {
		rs.end0()
	}
}

func (rs *RootSegmentSpan) AsyncFinish() {
	rs.DefaultSpan.AsyncFinish()
	rs.DefaultSpan.End(false)
	rs.end0()
}

func (rs *RootSegmentSpan) ContinueContext() {
	if !rs.InAsyncMode {
		panic("not in async mode")
	}
	context := getTracingContext()
	if context == nil {
		context = NewTracingContext()
	}
	saveSpanToActiveIfNotError(context, rs, nil)
}

func (rs *RootSegmentSpan) end0() {
	go func() {
		rs.doneCh <- atomic.SwapInt32(rs.SegmentContext.refNum, -1)
	}()
}

func (rs *RootSegmentSpan) createRootSegmentContext(ctx *TracingContext, _ SegmentSpan) (err error) {
	rs.SegmentID, err = GenerateGlobalID(ctx)
	if err != nil {
		return err
	}
	i := int32(0)
	rs.spanIDGenerator = &i
	rs.SpanID = i
	rs.ParentSpanID = -1
	return
}

type SnapshotSpan struct {
	DefaultSpan
	SegmentContext
}

func (s *SnapshotSpan) GetDefaultSpan() *DefaultSpan {
	return &s.DefaultSpan
}

func (s *SnapshotSpan) End() {
	panic(fmt.Errorf("cannot End the span in other goroutine"))
}

func (s *SnapshotSpan) SetOperationName(_ string) {
	panic(fmt.Errorf("cannot update the operation name of span in other goroutine"))
}

func (s *SnapshotSpan) SetSpanLayer(_ int32) {
	panic(fmt.Errorf("cannot update the layer of span in other goroutine"))
}

func (s *SnapshotSpan) SetComponent(_ int32) {
	panic(fmt.Errorf("cannot update the compoenent of span in other goroutine"))
}

func (s *SnapshotSpan) Tag(key, value string) {
	panic(fmt.Errorf("cannot add tag of span in other goroutine"))
}

func (s *SnapshotSpan) Log(_ ...string) {
	panic(fmt.Errorf("cannot add log of span in other goroutine"))
}

func (s *SnapshotSpan) Error(_ ...string) {
	panic(fmt.Errorf("cannot add error of span in other goroutine"))
}

func (s *SnapshotSpan) GetSegmentContext() SegmentContext {
	return s.SegmentContext
}

func (s *SnapshotSpan) tracer() *Tracer {
	return s.DefaultSpan.tracer
}

func (s *SnapshotSpan) segmentRegister() bool {
	for {
		o := atomic.LoadInt32(s.SegmentContext.refNum)
		if o < 0 {
			return false
		}
		if atomic.CompareAndSwapInt32(s.SegmentContext.refNum, o, o+1) {
			return true
		}
	}
}

func (s *SnapshotSpan) PrepareAsync() {
	panic("please use the PrepareAsync on right goroutine")
}

func (s *SnapshotSpan) AsyncFinish() {
	panic("please use the AsyncFinish on right goroutine")
}

func (s *SnapshotSpan) ContinueContext() {
	panic("please use the ContinueContext on right goroutine")
}

func newSegmentRoot(segmentSpan *SegmentSpanImpl) *RootSegmentSpan {
	s := &RootSegmentSpan{
		SegmentSpanImpl: segmentSpan,
	}
	var init int32
	s.refNum = &init
	ch := make(chan reporter.ReportedSpan)
	s.collect = ch
	s.notify = ch
	s.segment = make([]reporter.ReportedSpan, 0, 10)
	s.doneCh = make(chan int32)
	go func() {
		total := -1
		defer close(ch)
		defer close(s.doneCh)
		for {
			select {
			case span := <-s.notify:
				s.segment = append(s.segment, span)
			case n := <-s.doneCh:
				total = int(n)
			}
			if total == len(s.segment) {
				break
			}
		}
		s.tracer().Reporter.Send(append(s.segment, s))
	}()
	return s
}

func newSnapshotSpan(current TracingSpan) *SnapshotSpan {
	if current == nil {
		return nil
	}
	segmentSpan, ok := current.(SegmentSpan)
	if !ok || !segmentSpan.IsValid() { // is not segment span or segment is invalid(Executed End() method
		return nil
	}

	segCtx := segmentSpan.GetSegmentContext()
	copiedCorrelation := make(map[string]string)
	for k, v := range segCtx.CorrelationContext {
		copiedCorrelation[k] = v
	}
	s := &SnapshotSpan{
		DefaultSpan: DefaultSpan{
			OperationName: segmentSpan.GetOperationName(),
			Refs:          nil,
			tracer:        segmentSpan.tracer(),
			Peer:          segmentSpan.GetPeer(),
		},
		SegmentContext: SegmentContext{
			TraceID:            segCtx.GetTraceID(),
			SegmentID:          segCtx.SegmentID,
			SpanID:             segCtx.SpanID,
			collect:            segCtx.collect,
			refNum:             segCtx.refNum,
			spanIDGenerator:    segCtx.spanIDGenerator,
			FirstSpan:          segCtx.FirstSpan,
			CorrelationContext: copiedCorrelation,
		},
	}

	return s
}
