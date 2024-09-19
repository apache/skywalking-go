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

package metric

import (
	"github.com/apache/skywalking-go/plugins/core/metrics"
	"github.com/apache/skywalking-go/plugins/core/operator"
)

type CounterIncInterceptor struct{}

func (h *CounterIncInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	enhanced, ok := invocation.CallerInstance().(operator.EnhancedInstance)
	if !ok {
		return nil
	}

	counter, ok := enhanced.GetSkyWalkingDynamicField().(metrics.Counter)
	if ok && counter != nil {
		counter.Inc(invocation.Args()[0].(float64))
	}
	return nil
}

func (h *CounterIncInterceptor) AfterInvoke(_ operator.Invocation, _ ...interface{}) error {
	return nil
}
