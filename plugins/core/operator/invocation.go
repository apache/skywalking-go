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

type realInvocation struct {
	callerInstance interface{}
	args           []interface{}

	changeArgCallback func(int, interface{})

	isContinue   bool
	returnValues []interface{}

	context interface{}
	// owner is closed when the interceptor has finished, so the span it
	// stored with SetContext can be ended if it is still active. It is a
	// channel because this copied file cannot import sync/atomic.
	owner chan struct{}

	// self obs
	interTimeCost    int64
	beforeInterStart int64
}

func (i *realInvocation) CallerInstance() interface{} {
	return i.callerInstance
}

func (i *realInvocation) Args() []interface{} {
	return i.args
}

func (i *realInvocation) ChangeArg(index int, newValue interface{}) {
	i.changeArgCallback(index, newValue)
}

func (i *realInvocation) IsContinue() bool {
	return i.isContinue
}

func (i *realInvocation) DefineReturnValues(values ...interface{}) {
	i.isContinue = true
	i.returnValues = values
}

func (i *realInvocation) SetContext(context interface{}) {
	i.context = context
	if span, ok := context.(spanOwnerBinder); ok {
		if i.owner == nil {
			i.owner = make(chan struct{})
		}
		span.BindOwner(i.owner)
	}
}

// spanOwnerBinder is a span that can be bound to the invocation that ends it.
// Only builtin types are used, because every instrumented package has its own
// copy of this package.
type spanOwnerBinder interface {
	BindOwner(owner chan struct{})
}

type activeSpanMarker interface {
	ActiveSpanMark() (interface{}, int)
	RollbackActiveSpan(mark interface{}, depth int)
}

// markExited records that the interceptor of the invocation has finished, so
// the span it stored with SetContext must no longer be active.
func (i *realInvocation) markExited() {
	if i != nil && i.owner != nil {
		close(i.owner)
		i.owner = nil
	}
}

func spanMarker() activeSpanMarker {
	op := GetOperator()
	if op == nil {
		return nil
	}
	marker, _ := op.Tracing().(activeSpanMarker)
	return marker
}

// markActiveSpan returns the active span before BeforeInvoke runs.
func markActiveSpan() (mark interface{}, depth int) {
	if marker := spanMarker(); marker != nil {
		return marker.ActiveSpanMark()
	}
	return nil, 0
}

// rollbackActiveSpan ends the spans that a failed BeforeInvoke created and did
// not store with SetContext, so nothing else would end them.
func rollbackActiveSpan(i *realInvocation, mark interface{}, depth int) {
	// It runs in the recover of the before-hook too, where a panic would
	// reach the application.
	defer func() {
		_ = recover()
	}()
	if i == nil || i.context != nil || depth < 0 {
		// the after-hook still runs and ends the span it got, or the active
		// span was not marked
		return
	}
	if marker := spanMarker(); marker != nil {
		marker.RollbackActiveSpan(mark, depth)
	}
}

func (i *realInvocation) GetContext() interface{} {
	return i.context
}
