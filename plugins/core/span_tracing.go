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
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/apache/skywalking-go/plugins/core/reporter"

	commonv3 "github.com/apache/skywalking-go/protocols/collect/common/v3"
	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
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
	TraceID         string
	SegmentID       string
	SpanID          int32
	ParentSpanID    int32
	ParentSegmentID string
	collect         chan<- reporter.ReportedSpan
	// collectorDone is closed when the segment collector goroutine exits. Late
	// senders select on it instead of risking a send on a closed data channel
	// (receiving from a closed channel is always safe, sending never is).
	collectorDone      chan struct{}
	refNum             *int32
	spanIDGenerator    *int32
	FirstSpan          TracingSpan `json:"-"`
	CorrelationContext *CorrelationContext
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

func (c *SegmentContext) GetCorrelationContextValue(key string) string {
	return c.CorrelationContext.Get(key)
}

func (c *SegmentContext) SetCorrelationContextValue(key, value string) {
	c.CorrelationContext.Set(key, value)
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
	if s.DefaultSpan.endSyncAndFreeze() {
		s.end0()
	}
}

func (s *SegmentSpanImpl) AsyncFinish() {
	s.DefaultSpan.AsyncFinish()
	s.DefaultSpan.End(false)
	if s.DefaultSpan.endAndFreeze() {
		s.end0()
	}
}

func (s *SegmentSpanImpl) end0() {
	go func() {
		select {
		case s.SegmentContext.collect <- s:
		case <-s.SegmentContext.collectorDone:
			// The collector already exited (unreachable once the freeze
			// functions - endSyncAndFreeze/endAndFreeze - guarantee a single
			// end0 per span, kept as defense in depth): drop the span instead
			// of panicking on a closed channel send or blocking forever.
		}
	}()
}

func (s *SegmentSpanImpl) GetDefaultSpan() *DefaultSpan {
	return &s.DefaultSpan
}

// For Reported TracingSpan
//
// The ReportedSpan accessors below are intentionally NOT locked. Their safety
// rests on three guarantees that must be preserved together (re-review all of
// them before changing any one):
//  1. every mutator holds opLock and drops the write once ended==true;
//  2. endAndFreeze sets ended=true under opLock, so the span data can no
//     longer change after it returns;
//  3. the span reaches the collector goroutine through the end0 channel send,
//     whose happens-before edge publishes all pre-freeze writes to the reader.
// Therefore reporter.Transform always observes immutable data here.

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
			s.CorrelationContext = newCorrelationContextFrom(s.DefaultSpan.Refs[0].(*SpanContext).CorrelationContext)
		} else {
			s.TraceID, err = GenerateGlobalID(ctx)
			if err != nil {
				return err
			}
			s.CorrelationContext = newCorrelationContext()
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
		s.CorrelationContext = newCorrelationContext()
	}
	return
}

func (s *SegmentSpanImpl) IsProfileTarget() bool {
	return s.DefaultSpan.IsProfileTarget()
}

type RootSegmentSpan struct {
	*SegmentSpanImpl
	notify  <-chan reporter.ReportedSpan
	segment []reporter.ReportedSpan
	doneCh  chan int32
}

func (rs *RootSegmentSpan) End() {
	if rs.DefaultSpan.endSyncAndFreeze() {
		rs.end0()
	}
}

func (rs *RootSegmentSpan) AsyncFinish() {
	rs.DefaultSpan.AsyncFinish()
	rs.DefaultSpan.End(false)
	if rs.DefaultSpan.endAndFreeze() {
		rs.end0()
	}
}

func (rs *RootSegmentSpan) end0() {
	if rs == nil || rs.doneCh == nil || rs.SegmentContext.refNum == nil {
		return
	}
	select {
	case rs.doneCh <- atomic.SwapInt32(rs.SegmentContext.refNum, -1):
	case <-rs.SegmentContext.collectorDone:
		// see SegmentSpanImpl.end0
	}
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

func (rs *RootSegmentSpan) IsProfileTarget() bool {
	return rs.DefaultSpan.IsProfileTarget()
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

func (s *SnapshotSpan) ErrorOccured() {
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
	s.collectorDone = make(chan struct{})
	go func() {
		total := -1
		// Closing collectorDone (instead of the data channels) lets late
		// senders exit safely through their select; the unclosed channels are
		// reclaimed by the GC. It is closed right after the collect loop stops
		// receiving (so a late sender never blocks behind a slow
		// Reporter.SendTracing) and kept in a defer as well so that even a
		// panic below cannot leave senders blocked forever. Both call sites
		// run on this goroutine, so the plain bool needs no synchronization.
		doneClosed := false
		closeDone := func() {
			if !doneClosed {
				doneClosed = true
				close(s.collectorDone)
			}
		}
		defer closeDone()
		defer func() {
			// Defense in depth: a panic here would kill the process since this
			// goroutine has no other recover.
			if err := recover(); err != nil {
				defer func() { _ = recover() }() // a panicking logger must not re-kill us
				if tr := s.tracer(); tr != nil && tr.Log != nil {
					tr.Log.Errorf("segment collector panic: %v, stack: %s", err, debug.Stack())
				}
			}
		}()
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
		// the loop above is the only receiver: unblock late senders before the
		// (possibly slow) reporter call
		closeDone()
		s.tracer().Reporter.SendTracing(append(s.segment, s))
	}()
	return s
}

func newSnapshotSpan(current TracingSpan) TracingSpan {
	if current == nil {
		return nil
	}
	if _, isNoop := current.(*NoopSpan); isNoop {
		return newSnapshotNoopSpan()
	}
	segmentSpan, ok := current.(SegmentSpan)
	if !ok || !segmentSpan.IsValid() { // is not segment span or segment is invalid(Executed End() method
		return nil
	}

	segCtx := segmentSpan.GetSegmentContext()
	s := &SnapshotSpan{
		DefaultSpan: DefaultSpan{
			OperationName: segmentSpan.GetOperationName(),
			Refs:          nil,
			tracer:        segmentSpan.tracer(),
			Peer:          segmentSpan.GetPeer(),
			opLock:        &sync.Mutex{}, // keep the "opLock is never nil" invariant
		},
		SegmentContext: SegmentContext{
			TraceID:            segCtx.GetTraceID(),
			SegmentID:          segCtx.SegmentID,
			SpanID:             segCtx.SpanID,
			collect:            segCtx.collect,
			collectorDone:      segCtx.collectorDone,
			refNum:             segCtx.refNum,
			spanIDGenerator:    segCtx.spanIDGenerator,
			FirstSpan:          segCtx.FirstSpan,
			CorrelationContext: segCtx.CorrelationContext.Clone(),
		},
	}

	return s
}

func (s *SnapshotSpan) IsProfileTarget() bool {
	return s.DefaultSpan.IsProfileTarget()
}
