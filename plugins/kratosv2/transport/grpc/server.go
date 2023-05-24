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

	"github.com/go-kratos/kratos/v2/transport/grpc"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

var ignoreServerMiddlewareKey = "ignoreServerMiddleware"

type NewServerInterceptor struct {
}

func (n *NewServerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (n *NewServerInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	serverEnhanced, ok := results[0].(operator.EnhancedInstance)
	if !ok || serverEnhanced.GetSkyWalkingDynamicField() == true {
		return nil
	}
	server, ok := results[0].(*grpc.Server)
	if !ok {
		return nil
	}
	// adding the middleware to the server
	tracing.SetRuntimeContextValue(ignoreServerMiddlewareKey, true)
	grpc.Middleware(serverMiddleware)(server)
	serverEnhanced.SetSkyWalkingDynamicField(true)
	return nil
}
