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

package entry

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

func createExitSpan(caller interface{}, method string, opts ...tracing.SpanOption) (tracing.Span, error) {
	info := getInstanceInfo(caller)
	if info == nil {
		return nil, nil
	}

	span, err := tracing.CreateExitSpan(info.DBType()+"/"+method, info.Peer(), func(headerKey, headerValue string) error {
		return nil
	}, append(opts, tracing.WithComponent(info.ComponentID()),
		tracing.WithLayer(tracing.SpanLayerDatabase),
		tracing.WithTag(tracing.TagDBType, info.DBType()))...)
	return span, err
}

func createLocalSpan(caller interface{}, method string, opts ...tracing.SpanOption) (tracing.Span, InstanceInfo, error) {
	info := getInstanceInfo(caller)
	if info == nil {
		return nil, nil, nil
	}

	span, err := tracing.CreateLocalSpan(info.DBType()+"/"+method,
		append(opts, tracing.WithComponent(info.ComponentID()),
			tracing.WithLayer(tracing.SpanLayerDatabase),
			tracing.WithTag(tracing.TagDBType, info.DBType()))...)
	return span, info, err
}

func getInstanceInfo(caller interface{}) InstanceInfo {
	instance, ok := caller.(operator.EnhancedInstance)
	if !ok || instance == nil {
		return nil
	}
	df := instance.GetSkyWalkingDynamicField()
	if df == nil {
		return nil
	}
	info, ok := df.(InstanceInfo)
	if !ok {
		return nil
	}
	return info
}
