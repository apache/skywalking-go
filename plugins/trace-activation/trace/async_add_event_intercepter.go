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

package traceactivation

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
	"github.com/apache/skywalking-go/toolkit/trace"
)

type AsyncAddEventInterceptor struct {
}

func (h *AsyncAddEventInterceptor) BeforeInvoke(_ operator.Invocation) error {
	return nil
}

func (h *AsyncAddEventInterceptor) AfterInvoke(invocation operator.Invocation, _ ...interface{}) error {
	enhanced, ok := invocation.CallerInstance().(operator.EnhancedInstance)
	if !ok {
		return nil
	}
	s := enhanced.GetSkyWalkingDynamicField().(tracing.Span)
	et := invocation.Args()[0].(trace.EventType)
	event := invocation.Args()[1].(string)
	if len(event) == 0 {
		event = defaultEventMsg
	}
	s.Log(string(et), event)
	enhanced.SetSkyWalkingDynamicField(s)
	return nil
}
