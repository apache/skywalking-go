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
	"io"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ClientRecvMsgInterceptor struct {
}

func (h *ClientRecvMsgInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	cs := invocation.CallerInstance().(*nativeclientStream)
	method := cs.callHdr.Method
	if strings.HasPrefix(method, skywalkingService) {
		return nil
	}
	csEnhanced, ok := invocation.CallerInstance().(operator.EnhancedInstance)
	if ok && csEnhanced.GetSkyWalkingDynamicField() != nil {
		contextdata := csEnhanced.GetSkyWalkingDynamicField().(*contextData)
		tracing.ContinueContext(contextdata.continueSnapShot)
		contextdata.interceptFinish = true
	}
	s, err := tracing.CreateLocalSpan(formatOperationName(method, "/Client/Response/RecvMsg"),
		tracing.WithLayer(tracing.SpanLayerRPCFramework),
		tracing.WithTag(tracing.TagURL, method),
		tracing.WithComponent(23),
	)
	if err != nil {
		return err
	}
	invocation.SetContext(s)
	return nil
}

func (h *ClientRecvMsgInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	err, ok := result[0].(error)
	if ok && err != nil && err != io.EOF {
		span.Error(err.Error())
	}
	if err == io.EOF {
		cs := invocation.CallerInstance().(*nativeclientStream)
		method := cs.callHdr.Method
		span.SetOperationName(formatOperationName(method, "/Client/Response/CloseRecv"))
	}
	span.End()
	csEnhanced, ok := invocation.CallerInstance().(operator.EnhancedInstance)
	if ok && csEnhanced.GetSkyWalkingDynamicField() != nil {
		contextdata := csEnhanced.GetSkyWalkingDynamicField().(*contextData)
		tracing.ContinueContext(contextdata.endSnapShot)
	}
	return nil
}
