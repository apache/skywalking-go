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
)

type AddEventInterceptor struct {
}

func (h *AddEventInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	var (
		defaultEventType  tracing.EventType = "info"
		defaultEmptyEvent                   = "unknown"
	)

	span := tracing.ActiveSpan()
	if span != nil {
		et := invocation.Args()[0].(tracing.EventType)
		if len(et) == 0 {
			et = defaultEventType
		}
		event := invocation.Args()[1].(string)
		if len(event) == 0 {
			event = defaultEmptyEvent
		}
		span.Log(string(et), event)
	}
	return nil
}

func (h *AddEventInterceptor) AfterInvoke(_ operator.Invocation, _ ...interface{}) error {
	return nil
}
