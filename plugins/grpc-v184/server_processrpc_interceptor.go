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
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// ServerProcessRPCInterceptor intercepts (*Server).processRPC, which replaced
// processUnaryRPC and processStreamingRPC in grpc v1.84.0, and tags the entry
// span with the RPC type derived from the StreamDesc argument.
type ServerProcessRPCInterceptor struct {
}

func (h *ServerProcessRPCInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	activeSpan := tracing.ActiveSpan()
	if activeSpan == nil {
		return nil
	}
	args := invocation.Args()
	if len(args) < 4 {
		return nil
	}
	desc, ok := args[3].(*nativeStreamDesc)
	if !ok || desc == nil {
		return nil
	}
	activeSpan.Tag(RPCTypeTag, rpcTypeOf(desc))
	return nil
}

func (h *ServerProcessRPCInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}
