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

package server

import (
	"context"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tools"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"go-micro.dev/v4/metadata"
	"go-micro.dev/v4/server"
	"go-micro.dev/v4/transport"
)

var microComponentID int32 = 5009

type ServeRequestInterceptor struct {
}

func (n *ServeRequestInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	span, err := creatingSpan(invocation.Args()[0].(context.Context), invocation.Args()[1].(server.Request))
	if err != nil {
		return err
	}
	invocation.SetContext(span)
	return nil
}

func (n *ServeRequestInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	invocation.GetContext().(tracing.Span).End()
	return nil
}

func creatingSpan(ctx context.Context, req server.Request) (tracing.Span, error) {
	endpoint := req.Service() + "." + req.Endpoint()
	if s := getExistingSpan(req); s != nil {
		s.SetOperationName(endpoint)
		s.SetSpanLayer(tracing.SpanLayerRPCFramework)
		s.SetComponent(microComponentID)
		return s, nil
	}
	return tracing.CreateEntrySpan(endpoint, func(headerKey string) (string, error) {
		al, _ := metadata.Get(ctx, headerKey)
		return al, nil
	}, tracing.WithComponent(microComponentID),
		tracing.WithLayer(tracing.SpanLayerRPCFramework))
}

func getExistingSpan(req server.Request) tracing.Span {
	socketVal := tools.GetInstanceValueByType(req, tools.WithInterfaceType((*transport.Socket)(nil)))
	if socketVal == nil {
		return nil
	}
	instance, ok := socketVal.(operator.EnhancedInstance)
	if !ok || instance.GetSkyWalkingDynamicField() == nil {
		return nil
	}
	injected, ok := instance.GetSkyWalkingDynamicField().(*InjectData)
	if !ok {
		return nil
	}
	tracing.ContinueContext(injected.Snapshot)
	return injected.Span
}
