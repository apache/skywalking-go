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
	"google.golang.org/grpc/metadata"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ServerHandleStreamInterceptor struct {
}

func (h *ServerHandleStreamInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	stream := invocation.Args()[1].(*nativeStream)
	method := stream.Method()
	ctx := stream.Context()
	md, _ := metadata.FromIncomingContext(ctx)
	s, err := tracing.CreateEntrySpan(formatOperationName(method, ""), func(headerKey string) (string, error) {
		Value := ""
		vals := md.Get(headerKey)
		if len(vals) > 0 {
			Value = vals[0]
		}
		return Value, nil
	}, tracing.WithLayer(tracing.SpanLayerRPCFramework),
		tracing.WithTag(tracing.TagURL, method),
		tracing.WithComponent(23),
	)
	if err != nil {
		return err
	}
	invocation.SetContext(s)
	return nil
}

func (h *ServerHandleStreamInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	invocation.GetContext().(tracing.Span).End()
	return nil
}
