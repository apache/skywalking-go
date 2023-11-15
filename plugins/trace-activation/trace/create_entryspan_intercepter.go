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

package traceactivation

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
	"github.com/apache/skywalking-go/toolkit/trace"
)

type CreateEntrySpanInterceptor struct {
}

func (h *CreateEntrySpanInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (h *CreateEntrySpanInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	operationName := invocation.Args()[0].(string)
	var extractor func(headerKey string) (string, error) = invocation.Args()[1].(trace.ExtractorRef)
	s, err := tracing.CreateEntrySpan(operationName, extractor)
	if err != nil {
		invocation.DefineReturnValues(nil, err)
		return nil
	}
	enhancced, ok := result[0].(operator.EnhancedInstance)
	if !ok {
		return nil
	}
	enhancced.SetSkyWalkingDynamicField(s)
	return nil
}
