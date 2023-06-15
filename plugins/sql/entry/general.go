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
	"fmt"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type PrepareInfo struct {
	instance InstanceInfo
	span     tracing.Span
}

func GeneralPrepareBeforeInvoke(invocation operator.Invocation, method string) error {
	span, info, err := createLocalSpan(invocation.CallerInstance(), method,
		tracing.WithTag(tracing.TagDBStatement, invocation.Args()[1].(string)))
	if err != nil || span == nil {
		return err
	}
	invocation.SetContext(&PrepareInfo{
		instance: info,
		span:     span,
	})
	return nil
}

func GeneralPrepareAfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	ctx := invocation.GetContext()
	if ctx == nil {
		return nil
	}
	// if contains error, then record it
	if err, ok := results[1].(error); ok && err != nil {
		ctx.(*PrepareInfo).span.Error(err.Error())
	}
	ctx.(*PrepareInfo).span.End()

	// propagate the instance info
	if instance, ok := results[0].(*sql.Stmt); ok && instance != nil {
		results[0].(operator.EnhancedInstance).SetSkyWalkingDynamicField(ctx.(*PrepareInfo).instance)
	}
	return nil
}

type BeginTxInfo struct {
	instance InstanceInfo
	span     tracing.Span
}

func GeneralBeginTxBeforeInvoke(invocation operator.Invocation, method string) error {
	span, info, err := createLocalSpan(invocation.CallerInstance(), method)
	if err != nil || span == nil {
		return err
	}
	invocation.SetContext(&BeginTxInfo{
		instance: info,
		span:     span,
	})
	return nil
}

func GeneralBeginTxAfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	ctx := invocation.GetContext()
	if ctx == nil {
		return nil
	}
	// if contains error, then record it
	if err, ok := result[1].(error); ok && err != nil {
		ctx.(*BeginTxInfo).span.Error(err.Error())
	}
	ctx.(*BeginTxInfo).span.End()

	// propagate the instance info
	if instance, ok := result[0].(*sql.Tx); ok && instance != nil {
		result[0].(operator.EnhancedInstance).SetSkyWalkingDynamicField(ctx.(*BeginTxInfo).instance)
	}
	return nil
}

func GeneralExecBeforeInvoke(invocation operator.Invocation, method string) error {
	span, err := createExitSpan(invocation.CallerInstance(), method,
		tracing.WithTag(tracing.TagDBStatement, invocation.Args()[1].(string)))
	if err != nil {
		return err
	}
	if config.CollectParameter && len(invocation.Args()[2].([]interface{})) > 0 {
		span.Tag(tracing.TagDBSqlParameters, argsToString(invocation.Args()[2].([]interface{})))
	}
	invocation.SetContext(span)
	return nil
}

func GeneralExecAfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	ctx := invocation.GetContext()
	if ctx == nil {
		return nil
	}
	if err, ok := results[1].(error); ok && err != nil {
		ctx.(tracing.Span).Error(err.Error())
	}
	ctx.(tracing.Span).End()
	return nil
}

func GeneralPingBeforeInvoke(invocation operator.Invocation, method string) error {
	span, err := createExitSpan(invocation.CallerInstance(), method)
	if err != nil {
		return err
	}
	invocation.SetContext(span)
	return nil
}

func GeneralPingAfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	ctx := invocation.GetContext()
	if ctx == nil {
		return nil
	}
	if err, ok := results[0].(error); ok && err != nil {
		ctx.(tracing.Span).Error(err.Error())
	}
	ctx.(tracing.Span).End()
	return nil
}

func GeneralQueryBeforeInvoke(invocation operator.Invocation, method string) error {
	span, err := createExitSpan(invocation.CallerInstance(), method,
		tracing.WithTag(tracing.TagDBStatement, invocation.Args()[1].(string)))
	if err != nil {
		return err
	}
	if config.CollectParameter && len(invocation.Args()[2].([]interface{})) > 0 {
		span.Tag(tracing.TagDBSqlParameters, argsToString(invocation.Args()[2].([]interface{})))
	}
	invocation.SetContext(span)
	return nil
}

func GeneralQueryAfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	ctx := invocation.GetContext()
	if ctx == nil {
		return nil
	}
	if err, ok := results[1].(error); ok && err != nil {
		ctx.(tracing.Span).Error(err.Error())
	}
	ctx.(tracing.Span).End()
	return nil
}

func GeneralRawBeforeInvoke(invocation operator.Invocation, method string) error {
	span, err := createExitSpan(invocation.CallerInstance(), method)
	if err != nil {
		return err
	}
	invocation.SetContext(span)
	return nil
}

func GeneralRawAfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	ctx := invocation.GetContext()
	if ctx == nil {
		return nil
	}
	if err, ok := results[1].(error); ok && err != nil {
		ctx.(tracing.Span).Error(err.Error())
	}
	ctx.(tracing.Span).End()
	return nil
}

func argsToString(args []interface{}) string {
	switch len(args) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%v", args[0])
	}

	res := fmt.Sprintf("%v", args[0])
	for _, arg := range args[1:] {
		res += fmt.Sprintf(", %v", arg)
	}
	return res
}
