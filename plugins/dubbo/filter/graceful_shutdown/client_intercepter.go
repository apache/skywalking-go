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

// nolint
package graceful_shutdown

import (
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/protocol"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ClientInterceptor struct {
}

func (c *ClientInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	filterInvoker := invocation.Args()[1].(protocol.Invoker)
	dubboInv := invocation.Args()[2].(protocol.Invocation)
	url := filterInvoker.GetURL()
	if url == nil {
		return nil
	}
	s, err := tracing.CreateExitSpan(generateOperationName(filterInvoker, dubboInv), url.Location, func(k, v string) error {
		dubboInv.SetAttachment(k, v)
		return nil
	}, tracing.WithLayer(tracing.SpanLayerRPCFramework),
		tracing.WithTag(tracing.TagURL, url.String()),
		tracing.WithComponent(3))
	if err != nil {
		return err
	}
	invocation.SetContext(s)
	return nil
}

func (c *ClientInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if res, ok := results[0].(*protocol.RPCResult); ok && res.Error() != nil {
		span.Error(res.Error().Error())
	}

	span.End()
	return nil
}

func generateOperationName(invoker protocol.Invoker, inv protocol.Invocation) string {
	group := invoker.GetURL().GetParam(constant.GroupKey, "")
	if group != "" {
		group = "/" + group
	}
	return group + invoker.GetURL().Path + "/" + inv.MethodName()
}
