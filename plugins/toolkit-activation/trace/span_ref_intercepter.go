// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. The ASF licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traceactivation

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type SpanRefEndInterceptor struct {
}

func (h *SpanRefEndInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (h *SpanRefEndInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if span := spanFromRef(invocation); span != nil {
		span.End()
	}
	return nil
}

type SpanRefSetOperationNameInterceptor struct {
}

func (h *SpanRefSetOperationNameInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (h *SpanRefSetOperationNameInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if span := spanFromRef(invocation); span != nil {
		span.SetOperationName(invocation.Args()[0].(string))
	}
	return nil
}

func spanFromRef(invocation operator.Invocation) tracing.Span {
	enhanced, ok := invocation.CallerInstance().(operator.EnhancedInstance)
	if !ok {
		return nil
	}
	span, _ := enhanced.GetSkyWalkingDynamicField().(tracing.Span)
	return span
}
