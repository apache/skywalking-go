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

package grpc

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ClientFinishInterceptor struct {
}

func (h *ClientFinishInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	if tracing.GetRuntimeContextValue(interceptFinishMethod) != true {
		return nil
	}
	tracing.SetRuntimeContextValue(interceptFinishMethod, false)
	asyncSpan := tracing.GetRuntimeContextValue(ACTIVE_SPAN)
	if asyncSpan != nil {
		asyncSpan.(tracing.Span).AsyncFinish()
	}
	cs := invocation.CallerInstance().(*nativeclientStream)
	method := cs.callHdr.Method
	activeSpan := tracing.ActiveSpan()
	activeSpan.SetOperationName(formatOperationName(method, "/Client/Response/CloseRecv"))
	return nil
}

func (h *ClientFinishInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}
