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
	"math"
	"sync"
	"time"

	"github.com/apache/skywalking-go/plugins/core/reporter"

	commonv3 "github.com/apache/skywalking-go/protocols/collect/common/v3"
	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
)

type DefaultSpan struct {
	Refs          []reporter.SpanContext
	tracer        *Tracer
	StartTime     time.Time
	EndTime       time.Time
	OperationName string
	Peer          string
	Layer         agentv3.SpanLayer
	ComponentID   int32
	Tags          []*commonv3.KeyStringValuePair
	Logs          []*agentv3.Log
	IsError       bool
	SpanType      SpanType
	Parent        TracingSpan

	InAsyncMode       bool
	AsyncModeFinished bool

	// opLock guards the mutable fields above (OperationName, Peer, Layer,
	// ComponentID, Tags, Logs, Refs, IsError, EndTime, the async flags)
	// together with the ended flag. SpanType, Parent, StartTime and tracer are
	// write-once during construction - before the span is ever shared - and are
	// therefore read without the lock (IsEntry/IsExit/ParentSpan/StartTime).
	// Refs is also appended by ExtractContext; reporting reads it after the freeze.
	// It must stay a pointer: DefaultSpan is copied by value when it is embedded
	// into SegmentSpanImpl/SnapshotSpan, and an embedded sync.Mutex value would
	// trip the go vet copylocks check.
	opLock *sync.Mutex
	// ended is set by endAndFreeze right before the span is handed over to the
	// segment collector. Once set, every late mutation is dropped, so the span
	// data is frozen and the reporting path may read it without locking
	// (see the comment on the ReportedSpan accessors in span_tracing.go).
	ended bool
	// droppedLogged rate-limits logDroppedWrite to one warning per span: a
	// span leaked across goroutines would otherwise flood the log with one
	// line per dropped write.
	droppedLogged bool
	// reuseCount counts the extra logical owners that the span reuse rule in
	// CreateEntrySpan/CreateExitSpan hands this span to. Every owner calls End
	// exactly once and only the LAST End freezes and reports the span (see
	// enterReuse and endSyncAndFreeze). Guarded by opLock.
	reuseCount int
}

func NewDefaultSpan(tracer *Tracer, parent TracingSpan) *DefaultSpan {
	return &DefaultSpan{
		tracer:    tracer,
		StartTime: time.Now(),
		SpanType:  SpanTypeLocal,
		Parent:    parent,
		opLock:    &sync.Mutex{},
	}
}

// For TracingSpan
func (ds *DefaultSpan) SetOperationName(name string) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("operation name", name)
		return
	}
	ds.OperationName = name
}

func (ds *DefaultSpan) GetOperationName() string {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	return ds.OperationName
}

func (ds *DefaultSpan) SetPeer(peer string) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("peer", peer)
		return
	}
	ds.Peer = peer
}

func (ds *DefaultSpan) GetPeer() string {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	return ds.Peer
}

func (ds *DefaultSpan) SetSpanLayer(layer int32) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("span layer", "")
		return
	}
	ds.Layer = agentv3.SpanLayer(layer)
}

func (ds *DefaultSpan) GetSpanLayer() agentv3.SpanLayer {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	return ds.Layer
}

func (ds *DefaultSpan) SetComponent(componentID int32) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("component", "")
		return
	}
	ds.ComponentID = componentID
}

func (ds *DefaultSpan) GetComponent() int32 {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	return ds.ComponentID
}

func (ds *DefaultSpan) Tag(key, value string) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("tag", key)
		return
	}
	for _, tag := range ds.Tags {
		if tag.Key == key {
			tag.Value = value
			return
		}
	}
	ds.Tags = append(ds.Tags, &commonv3.KeyStringValuePair{Key: key, Value: value})
}

// log0 is the lock-free internal implementation of Log: Error reuses it while
// already holding opLock, avoiding a re-entrant deadlock.
func (ds *DefaultSpan) log0(ll ...string) {
	data := make([]*commonv3.KeyStringValuePair, 0, int32(math.Ceil(float64(len(ll))/2.0)))
	var kvp *commonv3.KeyStringValuePair
	for i, l := range ll {
		if i%2 == 0 {
			kvp = &commonv3.KeyStringValuePair{}
			data = append(data, kvp)
			kvp.Key = l
		} else if kvp != nil {
			kvp.Value = l
		}
	}
	ds.Logs = append(ds.Logs, &agentv3.Log{Time: Millisecond(time.Now()), Data: data})
}

func (ds *DefaultSpan) Log(ll ...string) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("log", "")
		return
	}
	ds.log0(ll...)
}

func (ds *DefaultSpan) Error(ll ...string) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("error", "")
		return
	}
	ds.IsError = true
	ds.log0(ll...)
}

func (ds *DefaultSpan) ErrorOccured() {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("error flag", "")
		return
	}
	ds.IsError = true
}

// logDroppedWrite reports a mutation that arrived after the span was frozen.
// The caller must hold opLock (reading OperationName/droppedLogged is therefore
// safe). The span name (e.g. "GET:/api/xxx", "MySQL/query") lets users locate
// which plugin or code path is still writing a finished span, which usually
// means the span leaked across goroutines. Only the FIRST drop per span is
// logged so a leaked hot span cannot flood the log.
func (ds *DefaultSpan) logDroppedWrite(op, detail string) {
	if ds.droppedLogged {
		return
	}
	ds.droppedLogged = true
	if ds.tracer != nil && ds.tracer.Log != nil {
		ds.tracer.Log.Warnf(
			"span %q already ended, dropping %s %q (and any further late writes; span shared across goroutines?)",
			ds.OperationName, op, detail)
	}
}

func (ds *DefaultSpan) End(changeParent bool) {
	ds.opLock.Lock()
	if !ds.ended {
		ds.EndTime = time.Now()
	}
	ds.opLock.Unlock()
	// The remaining work is goroutine-local bookkeeping, not shared span data.
	GetSo11y(ds.tracer).MeasureTracingContextCompletion(false)
	if changeParent {
		if ctx := getTracingContext(); ctx != nil {
			ctx.SaveActiveSpan(ds.Parent)
		}
	}
}

// endAndFreeze marks the span as ended under the lock. It returns true only on
// the first call, so the caller can ensure end0() runs exactly once even when
// End is raced from multiple goroutines (no duplicated segment reporting).
// After it returns, every late mutator observes ended==true and drops its
// write, so the span data is frozen: the reporting path reads it without locks
// relying on the channel handoff in end0 for the happens-before edge.
func (ds *DefaultSpan) endAndFreeze() bool {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		return false
	}
	ds.ended = true
	return true
}

// appendRef attaches one more segment reference to this span (see
// Tracer.ExtractContext); late appends after the freeze are dropped.
func (ds *DefaultSpan) appendRef(ref reporter.SpanContext) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		ds.logDroppedWrite("ref", "")
		return
	}
	ds.Refs = append(ds.Refs, ref)
}

// enterReuse registers one more owner of this span. It is called from the span
// reuse branches of CreateEntrySpan/CreateExitSpan when the active span is
// handed to a nested plugin; that owner's End then only decrements the counter
// (see endSyncAndFreeze) instead of freezing the span.
func (ds *DefaultSpan) enterReuse() {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if !ds.ended {
		ds.reuseCount++
	}
}

// endSyncAndFreeze performs the whole synchronous End() path in ONE critical
// section - the "already ended" fast check, the EndTime write and the freeze
// decision used to take three separate lock round-trips per span end. It
// returns true when the caller must hand the span to the collector (exactly
// once, and never for async spans: AsyncFinish owns the freeze there, and
// reading InAsyncMode under the same lock keeps even a misused concurrent
// PrepareAsync/End pair free of data races).
func (ds *DefaultSpan) endSyncAndFreeze() bool {
	ds.opLock.Lock()
	if ds.reuseCount > 0 {
		// a nested owner of a reused span (see enterReuse) finished: this is
		// not the last End, so keep the span open - the outer owner still
		// writes to it (e.g. gorm tags db.statement after the sql driver's
		// End) and will freeze it with its own End. Restoring the active span
		// is kept here because the nested plugin expects its End to pop the
		// span, exactly like before the freeze mechanism.
		ds.reuseCount--
		ds.opLock.Unlock()
		if ctx := getTracingContext(); ctx != nil {
			ctx.SaveActiveSpan(ds.Parent)
		}
		return false
	}
	if !ds.EndTime.IsZero() { // already ended
		ds.opLock.Unlock()
		return false
	}
	ds.EndTime = time.Now()
	frozen := false
	// EndTime and ended are distinct on purpose: in async mode End() sets
	// EndTime but leaves ended=false - AsyncFinish owns the freeze there.
	if !ds.InAsyncMode && !ds.ended {
		ds.ended = true
		frozen = true
	}
	ds.opLock.Unlock()
	// goroutine-local bookkeeping stays outside the lock
	GetSo11y(ds.tracer).MeasureTracingContextCompletion(false)
	if ctx := getTracingContext(); ctx != nil {
		ctx.SaveActiveSpan(ds.Parent)
	}
	return frozen
}

func (ds *DefaultSpan) IsEntry() bool {
	return ds.SpanType == SpanTypeEntry
}

func (ds *DefaultSpan) IsExit() bool {
	return ds.SpanType == SpanTypeExit
}

func (ds *DefaultSpan) IsValid() bool {
	// EndTime may be written by another goroutine in async mode, take the lock
	// to avoid a torn read of the time.Time value.
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	return ds.EndTime.IsZero()
}

func (ds *DefaultSpan) ParentSpan() TracingSpan {
	return ds.Parent
}

func (ds *DefaultSpan) PrepareAsync() {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		// the span is already frozen and reported; entering async mode now is
		// a misuse and would only confuse the lifecycle, so drop it
		ds.logDroppedWrite("prepare async", "")
		return
	}
	if ds.InAsyncMode {
		panic("already in async mode")
	}
	ds.InAsyncMode = true
	ds.AsyncModeFinished = false
}

func (ds *DefaultSpan) AsyncFinish() {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if ds.ended {
		// already frozen and reported (the matching PrepareAsync was dropped,
		// or another finisher won the race): drop, mirroring the mutator
		// policy, instead of panicking on an already-completed span
		ds.logDroppedWrite("async finish", "")
		return
	}
	if !ds.InAsyncMode {
		panic("not in async mode")
	}
	if ds.AsyncModeFinished {
		panic("already finished async")
	}
	ds.AsyncModeFinished = true
}

// GetEndPointName must not be called while holding opLock (it locks through
// GetOperationName, mirroring how Error must use log0 instead of Log).
func (ds *DefaultSpan) GetEndPointName() string {
	if ds.SpanType == SpanTypeEntry {
		return ds.GetOperationName()
	}
	return ""
}

func (ds *DefaultSpan) GetParentSpan() interface{} {
	return ds.Parent
}

func (ds *DefaultSpan) IsProfileTarget() bool {
	endPoint := ds.GetEndPointName()
	if ds.tracer.ProfileManager.IfProfiling() {
		return ds.tracer.ProfileManager.CheckIfProfileTarget(endPoint)
	}
	return false
}
