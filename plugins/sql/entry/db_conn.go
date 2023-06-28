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

type ConnInterceptor struct {
}

type connInfo struct {
	instance InstanceInfo
	span     tracing.Span
}

func (n *ConnInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	span, info, err := createLocalSpan(invocation.CallerInstance(), "Conn")
	if err != nil || span == nil {
		return err
	}
	invocation.SetContext(&connInfo{
		instance: info,
		span:     span,
	})
	return nil
}

func (n *ConnInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	ctx := invocation.GetContext()
	if ctx == nil {
		return nil
	}
	// if contains error, then record it
	if err, ok := results[1].(error); ok && err != nil {
		ctx.(*connInfo).span.Error(err.Error())
	}
	ctx.(*connInfo).span.End()

	// propagate the instance info
	if instance, ok := results[0].(*sql.Conn); ok && instance != nil {
		results[0].(operator.EnhancedInstance).SetSkyWalkingDynamicField(ctx.(*connInfo).instance)
	}
	return nil
}
