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

package socket

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

//skywalking:public
type InjectData struct {
	Span     tracing.Span
	Snapshot tracing.ContextSnapshot
}

type AcceptInterceptor struct {
}

func (n *AcceptInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (n *AcceptInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	// error != nil then ignore
	if results[0] != nil {
		return nil
	}
	span := tracing.ActiveSpan()
	if span == nil {
		return nil
	}
	instance := invocation.CallerInstance().(operator.EnhancedInstance)
	if _, existingData := instance.GetSkyWalkingDynamicField().(*InjectData); existingData {
		return nil
	}

	span.PrepareAsync()
	context := tracing.CaptureContext()
	span.End()
	instance.SetSkyWalkingDynamicField(&InjectData{
		Span:     span,
		Snapshot: context,
	})
	return nil
}
