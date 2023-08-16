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

type ServerRecvMsgInterceptor struct {
}

func (h *ServerRecvMsgInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	ss := invocation.CallerInstance().(*nativeserverStream)
	method := ss.s.Method()
	if strings.HasPrefix(method, "/skywalking") {
		return nil
	}
	s, err := tracing.CreateLocalSpan(formatOperationName(method, "/Server/Response/RecvMsg"),
		tracing.WithLayer(tracing.SpanLayerRPCFramework),
		tracing.WithTag(tracing.TagURL, method),
		tracing.WithComponent(23),
	)
	invocation.SetContext(s)
	if err != nil {
		return err
	}
	return nil
}

func (h *ServerRecvMsgInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	err, ok := result[0].(error)
	if ok && err != nil && err != io.EOF {
		span.Error(err.Error())
	}
	if err == io.EOF {
		ss := invocation.CallerInstance().(*nativeserverStream)
		method := ss.s.Method()
		span.SetOperationName(formatOperationName(method, "/Server/Response/CloseRecv"))
	}
	span.End()
	return nil
}
