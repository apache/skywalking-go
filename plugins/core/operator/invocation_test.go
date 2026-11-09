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

package operator

import "testing"

type ownedSpan struct {
	owners []chan struct{}
}

func closed(owner chan struct{}) bool {
	select {
	case <-owner:
		return true
	default:
		return false
	}
}

func (s *ownedSpan) BindOwner(owner chan struct{}) {
	s.owners = append(s.owners, owner)
}

// fakeTracing records the calls of the template helpers.
type fakeTracing struct {
	mark      interface{}
	depth     int
	rollbacks []int
}

func (f *fakeTracing) ActiveSpanMark() (mark interface{}, depth int) {
	return f.mark, f.depth
}

func (f *fakeTracing) RollbackActiveSpan(_ interface{}, depth int) {
	f.rollbacks = append(f.rollbacks, depth)
}

type fakeOperator struct {
	Operator
	tracing interface{}
}

func (f *fakeOperator) Tracing() interface{} {
	return f.tracing
}

func withOperator(t *testing.T, op Operator) {
	t.Helper()
	old := GetOperator
	GetOperator = func() Operator { return op }
	t.Cleanup(func() { GetOperator = old })
}

func TestSetContextBindsSpanToInvocation(t *testing.T) {
	inv := &realInvocation{}
	span := &ownedSpan{}
	inv.SetContext(span)
	inv.SetContext(span)
	if len(span.owners) != 2 || span.owners[0] == nil || span.owners[0] != span.owners[1] {
		t.Fatalf("every SetContext should bind the same owner of the invocation, got %v", span.owners)
	}
	if closed(span.owners[0]) {
		t.Fatal("the owner should not be finished before the interceptor finishes")
	}
	inv.markExited()
	if !closed(span.owners[0]) {
		t.Fatal("markExited should finish the owner")
	}
	// the after-hook and the skip path may both finish the invocation
	inv.markExited()
}

func TestSetContextWithoutSpanHasNoOwner(t *testing.T) {
	inv := &realInvocation{}
	inv.SetContext("not a span")
	inv.SetContext(nil)
	inv.markExited()
	if inv.owner != nil {
		t.Fatal("a context that is not a span should not create an owner")
	}
	if inv.GetContext() != nil {
		t.Fatal("SetContext should still store the context")
	}
	var nilInvocation *realInvocation
	nilInvocation.markExited()
}

func TestMarkActiveSpanWithoutTracer(t *testing.T) {
	withOperator(t, nil)
	if mark, depth := markActiveSpan(); mark != nil || depth != 0 {
		t.Fatalf("no tracer should mark nothing, got %v %d", mark, depth)
	}
	withOperator(t, &fakeOperator{tracing: struct{}{}})
	if mark, depth := markActiveSpan(); mark != nil || depth != 0 {
		t.Fatalf("a tracer without marks should mark nothing, got %v %d", mark, depth)
	}
	rollbackActiveSpan(&realInvocation{}, nil, 0)
}

func TestRollbackActiveSpan(t *testing.T) {
	tracer := &fakeTracing{mark: "active", depth: 2}
	withOperator(t, &fakeOperator{tracing: tracer})
	mark, depth := markActiveSpan()
	if mark != "active" || depth != 2 {
		t.Fatalf("markActiveSpan should return the tracer mark, got %v %d", mark, depth)
	}

	rollbackActiveSpan(nil, mark, depth)
	rollbackActiveSpan(&realInvocation{}, mark, -1)
	withContext := &realInvocation{}
	withContext.SetContext("context")
	rollbackActiveSpan(withContext, mark, depth)
	if len(tracer.rollbacks) != 0 {
		t.Fatalf("no rollback without an invocation, without a mark, or with a context, got %v", tracer.rollbacks)
	}

	rollbackActiveSpan(&realInvocation{}, mark, depth)
	if len(tracer.rollbacks) != 1 || tracer.rollbacks[0] != 2 {
		t.Fatalf("a failed hook without a context should roll back to the mark, got %v", tracer.rollbacks)
	}
}
