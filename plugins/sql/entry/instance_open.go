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
	"database/sql"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

var needInfoKey = "needInfo"
var infoKey = "info"

type InstanceInterceptor struct {
}

type InstanceInfo interface {
	Peer() string
	ComponentID() int32
	DBType() string
}

func (n *InstanceInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	tracing.SetRuntimeContextValue(needInfoKey, true)
	return nil
}

func (n *InstanceInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	tracing.SetRuntimeContextValue(needInfoKey, nil)
	info, ok := tracing.GetRuntimeContextValue(infoKey).(InstanceInfo)
	tracing.SetRuntimeContextValue(needInfoKey, nil)
	tracing.SetRuntimeContextValue(infoKey, nil)
	if !ok || info == nil {
		return nil
	}

	// adding peer address into db
	if db, ok := results[0].(*sql.DB); ok && db != nil {
		results[0].(operator.EnhancedInstance).SetSkyWalkingDynamicField(info)
	}
	return nil
}
