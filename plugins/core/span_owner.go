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

// maxLeakedSpans bounds the loops that end leaked spans, in case a span does
// not leave the active position when it ends.
const maxLeakedSpans = 64

// noOwner is the owner of a noop span that its creator nested before storing
// it with SetContext: it is never closed.
var noOwner = make(chan struct{})

// BindOwner binds the span to the invocation that stores it with SetContext.
// While the span is reused, the nested owner that stores it marks its reuse
// as stored instead (see endSyncAndFreeze), so only the creator binds.
func (ds *DefaultSpan) BindOwner(owner chan struct{}) {
	ds.opLock.Lock()
	defer ds.opLock.Unlock()
	if n := len(ds.reuses); n > 0 {
		ds.reuses[n-1] = true
		return
	}
	if ds.owner == nil {
		ds.owner = owner
	}
}

// BindOwner does nothing: a snapshot span is owned by another goroutine.
func (s *SnapshotSpan) BindOwner(chan struct{}) {
}

// BindOwner binds the noop span to the invocation that stores it with SetContext.
func (n *NoopSpan) BindOwner(owner chan struct{}) {
	if n.owner == nil {
		n.owner = owner
	}
}

// isClosed reports whether the owner of a span has finished. A nil channel,
// a span without an owner, is never closed.
func isClosed(owner chan struct{}) bool {
	select {
	case <-owner:
		return true
	default:
		return false
	}
}

// spanOwnerFinished reports whether the interceptor that stored the span has
// finished, so nothing will end the span anymore. A span that no interceptor
// stored, such as a span of the toolkit API or of a gorm callback, has no owner.
func spanOwnerFinished(span TracingSpan) bool {
	switch s := span.(type) {
	case *NoopSpan:
		return isClosed(s.owner)
	case *SnapshotSpan:
		return false
	case SegmentSpan:
		ds := s.GetDefaultSpan()
		ds.opLock.Lock()
		owner := ds.owner
		ds.opLock.Unlock()
		return isClosed(owner)
	}
	return false
}

// spanDepth is how many times the span was handed out again while it was
// active: the nested spans of a noop span, or the owners of a reused span.
func spanDepth(span TracingSpan) int {
	switch s := span.(type) {
	case *NoopSpan:
		return s.stackCount
	case *SnapshotSpan:
		return 0
	case SegmentSpan:
		ds := s.GetDefaultSpan()
		ds.opLock.Lock()
		defer ds.opLock.Unlock()
		return len(ds.reuses)
	}
	return 0
}

// restoreDepth undoes the nesting that was added after the depth was taken.
func restoreDepth(span TracingSpan, depth int) {
	switch s := span.(type) {
	case *NoopSpan:
		if s.stackCount > depth {
			s.stackCount = depth
		}
	case *SnapshotSpan:
	case SegmentSpan:
		ds := s.GetDefaultSpan()
		ds.opLock.Lock()
		if len(ds.reuses) > depth {
			ds.reuses = ds.reuses[:depth]
		}
		ds.opLock.Unlock()
	}
}

// endLeakedSpan ends the active span of ctx for a plugin that cannot end it
// anymore, and makes its parent the active span.
func (t *Tracer) endLeakedSpan(ctx *TracingContext, span TracingSpan) {
	switch s := span.(type) {
	case *NoopSpan:
		GetSo11y(t).MeasureLeakedTracingContext(true)
		// the nested spans that never ended are ended together with it
		s.stackCount = 1
		s.End()
	case *SnapshotSpan:
		// owned by another goroutine, and its End panics: only leave it
	case SegmentSpan:
		GetSo11y(t).MeasureLeakedTracingContext(false)
		ds := s.GetDefaultSpan()
		// the owners that never ended are ended together with it, so this
		// End reports the span and its segment is not held back
		ds.opLock.Lock()
		ds.reuses = nil
		name := ds.OperationName
		ds.opLock.Unlock()
		if t.Log != nil {
			t.Log.Warnf("ending the span %q that its plugin did not end", name)
		}
		span.End()
	}
	if ctx.ActiveSpan() == span {
		ctx.SaveActiveSpan(span.ParentSpan())
	}
}

// endOrphanSpans ends the active spans whose interceptor has finished without
// ending them, and returns the active span that is left. Such a span would
// otherwise stay active on the goroutine, and every later request on it would
// become its child, or be ignored under a noop span.
func (t *Tracer) endOrphanSpans(ctx *TracingContext) TracingSpan {
	for i := 0; i < maxLeakedSpans; i++ {
		span := ctx.ActiveSpan()
		if span == nil || !spanOwnerFinished(span) {
			return span
		}
		t.endLeakedSpan(ctx, span)
	}
	return ctx.ActiveSpan()
}

// ActiveSpanMark returns the active span and its depth before a before-hook
// runs, for RollbackActiveSpan.
func (t *Tracer) ActiveSpanMark() (mark interface{}, depth int) {
	ctx := getTracingContext()
	if ctx == nil {
		return nil, 0
	}
	// the spans that the before-hook creates replace the orphans first, so
	// the mark must not be an orphan
	span := t.endOrphanSpans(ctx)
	if span == nil {
		return nil, 0
	}
	return span, spanDepth(span)
}

// RollbackActiveSpan ends the spans that a failed before-hook created and did
// not store with SetContext, back to the active span of ActiveSpanMark. A
// context that the before-hook continued, such as the snapshot of a stream,
// is kept, and only the spans created on it are ended. It changes nothing when
// neither is an ancestor of the active span, because then the before-hook did
// more than create spans.
func (t *Tracer) RollbackActiveSpan(mark interface{}, depth int) {
	ctx := getTracingContext()
	if ctx == nil {
		return
	}
	markSpan, _ := mark.(TracingSpan)
	if !reachesRollbackBoundary(markSpan, ctx.ActiveSpan()) {
		return
	}
	for i := 0; i < maxLeakedSpans; i++ {
		span := ctx.ActiveSpan()
		if span == nil {
			return
		}
		if span == markSpan {
			restoreDepth(span, depth)
			return
		}
		if isContinuedSnapshot(span) {
			return
		}
		t.endLeakedSpan(ctx, span)
	}
}

// isContinuedSnapshot reports whether the span stands for a context captured
// on another goroutine: a snapshot span, or the noop span of an ignored or
// sampled-out snapshot.
func isContinuedSnapshot(span TracingSpan) bool {
	switch s := span.(type) {
	case *SnapshotSpan:
		return true
	case *NoopSpan:
		return s.snapshot
	}
	return false
}

// reachesRollbackBoundary reports whether the mark, or a continued snapshot,
// is the span or one of its parents. A nil mark is the parent of every root
// span.
func reachesRollbackBoundary(mark, span TracingSpan) bool {
	for i := 0; i < maxLeakedSpans; i++ {
		if span == mark || (span != nil && isContinuedSnapshot(span)) {
			return true
		}
		if span == nil {
			return false
		}
		span = span.ParentSpan()
	}
	return false
}
