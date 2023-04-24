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
	"github.com/apache/skywalking-go/plugins/core/operator"
)

var (
	errParameter = operator.NewError("parameter are nil")
)

// CreateEntrySpan creates a new entry span.
// operationName is the name of the span.
// extractor is the extractor to extract the context from the carrier.
// opts is the options to create the span.
func CreateEntrySpan(operationName string, extractor Extractor, opts ...SpanOption) (s Span, err error) {
	if operationName == "" || extractor == nil {
		return nil, errParameter
	}
	op := operator.GetOperator()
	if op == nil {
		return &NoopSpan{}, nil
	}
	span, err := op.Tracing().(operator.TracingOperator).CreateEntrySpan(operationName, extractorWrapper(extractor), copyOptsAsInterface(opts)...)
	if err != nil {
		return nil, err
	}
	return newSpanAdapter(span.(AdaptSpan)), nil
}

// CreateLocalSpan creates a new local span.
// operationName is the name of the span.
// opts is the options to create the span.
func CreateLocalSpan(operationName string, opts ...SpanOption) (s Span, err error) {
	if operationName == "" {
		return nil, errParameter
	}
	op := operator.GetOperator()
	if op == nil {
		return &NoopSpan{}, nil
	}
	span, err := op.Tracing().(operator.TracingOperator).CreateLocalSpan(operationName, copyOptsAsInterface(opts)...)
	if err != nil {
		return nil, err
	}
	return newSpanAdapter(span.(AdaptSpan)), nil
}

// CreateExitSpan creates a new exit span.
// operationName is the name of the span.
// peer is the peer address of the span.
// injector is the injector to inject the context into the carrier.
// opts is the options to create the span.
func CreateExitSpan(operationName, peer string, injector Injector, opts ...SpanOption) (s Span, err error) {
	if operationName == "" || peer == "" || injector == nil {
		return nil, errParameter
	}
	op := operator.GetOperator()
	if op == nil {
		return &NoopSpan{}, nil
	}
	span, err := op.Tracing().(operator.TracingOperator).CreateExitSpan(operationName, peer, injectorWrapper(injector), copyOptsAsInterface(opts)...)
	if err != nil {
		return nil, err
	}
	return newSpanAdapter(span.(AdaptSpan)), nil
}

// ActiveSpan returns the current active span, it can be got the current span in the current goroutine.
// If the current goroutine is not in the context of the span, it will return nil.
// If get the span from other goroutine, it can only get information but cannot be operated.
func ActiveSpan() Span {
	op := operator.GetOperator()
	if op == nil {
		return nil
	}
	if span, ok := op.Tracing().(operator.TracingOperator).ActiveSpan().(AdaptSpan); ok {
		return newSpanAdapter(span)
	}
	return nil
}

// GetRuntimeContextValue returns the value of the key in the runtime context, which is current goroutine.
// The value can also read from the goroutine which is created by the current goroutine
func GetRuntimeContextValue(key string) interface{} {
	op := operator.GetOperator()
	if op == nil {
		return nil
	}
	return op.Tracing().(operator.TracingOperator).GetRuntimeContextValue(key)
}

// SetRuntimeContextValue sets the value of the key in the runtime context.
func SetRuntimeContextValue(key string, val interface{}) {
	op := operator.GetOperator()
	if op != nil {
		op.Tracing().(operator.TracingOperator).SetRuntimeContextValue(key, val)
	}
}

func copyOptsAsInterface(opts []SpanOption) []interface{} {
	optsVal := make([]interface{}, len(opts))
	for i := range opts {
		optsVal[i] = opts[i]
	}
	return optsVal
}

type extractorWrapperImpl struct {
	extractor Extractor
}

func (e *extractorWrapperImpl) Fun() func(headerKey string) (string, error) {
	return e.extractor
}

func extractorWrapper(extractor Extractor) *extractorWrapperImpl {
	return &extractorWrapperImpl{extractor: extractor}
}

type injectorWrapperImpl struct {
	injector Injector
}

func (i *injectorWrapperImpl) Fun() func(headerKey, headerValue string) error {
	return i.injector
}

func injectorWrapper(injector Injector) *injectorWrapperImpl {
	return &injectorWrapperImpl{injector: injector}
}

type NoopSpan struct {
}

func (n *NoopSpan) SetOperationName(string) {
}
func (n *NoopSpan) SetPeer(string) {
}
func (n *NoopSpan) SetSpanLayer(SpanLayer) {
}
func (n *NoopSpan) SetComponent(int32) {
}
func (n *NoopSpan) Tag(Tag, string) {
}
func (n *NoopSpan) Log(...string) {
}
func (n *NoopSpan) Error(...string) {
}
func (n *NoopSpan) End() {
}
