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
	"context"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ClientStreamingInterceptor struct {
}

func (h *ClientStreamingInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	method := invocation.Args()[2].(string)
	clientconn := invocation.CallerInstance().(*nativeClientConn)
	ctx := invocation.Args()[0].(context.Context)
	remoteAddr := clientconn.Target()
	if strings.HasPrefix(method, "/skywalking") {
		return nil
	}
	s, err := tracing.CreateExitSpan(formatOperationName(method, ""), remoteAddr, func(headerKey, headerValue string) error {
		ctx = metadata.AppendToOutgoingContext(ctx, headerKey, headerValue)
		invocation.ChangeArg(0, ctx)
		return nil
	},
		tracing.WithLayer(tracing.SpanLayerRPCFramework),
		tracing.WithTag(tracing.TagURL, method),
		tracing.WithComponent(23),
	)
	if err != nil {
		return err
	}
	s.Tag(RPC_TYPE_TAG, "Streaming")
	s.PrepareAsync()
	tracing.SetRuntimeContextValue(ACTIVE_SPAN, s)
	tracing.SetRuntimeContextValue(RPC_TYPE, "Streaming")
	tracing.SetRuntimeContextValue(CONTINUE_SNAPSHOT, tracing.CaptureContext())
	invocation.SetContext(s)
	return nil
}

func (h *ClientStreamingInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if err, ok := result[0].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	tracing.SetRuntimeContextValue(END_SNAPSHOT, tracing.CaptureContext())
	return nil
}
