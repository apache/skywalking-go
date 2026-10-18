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

package grpcv184

import (
	"strings"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ServerSendMsgInterceptor struct {
}

func (h *ServerSendMsgInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	ss := invocation.CallerInstance().(*nativeserverStream)
	method := ss.s.Method()
	operation := "/Server/Request/SendMsg"
	if isUnaryStreamDesc(ss.desc) {
		// Since grpc v1.84.0 the unary response is sent through serverStream.SendMsg
		// instead of (*Server).sendResponse, so create the span that grpc < v1.84.0
		// created in sendResponse.
		if tracing.ActiveSpan() == nil {
			return nil
		}
		operation = "/Server/Response/SendResponse"
	} else if strings.HasPrefix(method, skywalkingService) {
		return nil
	}
	s, err := tracing.CreateLocalSpan(formatOperationName(method, operation),
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

func (h *ServerSendMsgInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if err, ok := result[0].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
