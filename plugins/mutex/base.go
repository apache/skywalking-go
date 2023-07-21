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

package mutex

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

var componentID int32 = 5016

var lockingIsExecutingKey = "lockingIsExecuting"

func BeforeInvoke(invocation operator.Invocation, name string) error {
	// must have tracing context
	span := tracing.ActiveSpan()
	if span == nil {
		return nil
	}
	// ignore if already in locking span(avoid recursive call)
	if isLocking := tracing.GetRuntimeContextValue(lockingIsExecutingKey); isLocking != nil {
		return nil
	}
	s, err := tracing.CreateLocalSpan(name, tracing.WithComponent(componentID))
	if err != nil {
		return err
	}
	tracing.SetRuntimeContextValue(lockingIsExecutingKey, true)
	invocation.SetContext(s)
	return nil
}

func AfterInvoke(invocation operator.Invocation) error {
	if invocation.GetContext() == nil {
		return nil
	}
	invocation.GetContext().(tracing.Span).End()
	tracing.SetRuntimeContextValue(lockingIsExecutingKey, nil)
	return nil
}

func AfterInvokeWithTag(invocation operator.Invocation, tagKey, tagValue string) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	span.Tag(tagKey, tagValue)
	span.End()
	tracing.SetRuntimeContextValue(lockingIsExecutingKey, nil)
	return nil
}
