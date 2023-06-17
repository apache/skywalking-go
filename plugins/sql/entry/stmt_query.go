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

type StmtQueryInterceptor struct {
}

func (n *StmtQueryInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	span, err := createExitSpan(invocation.CallerInstance(), "Stmt/Query")
	if err != nil {
		return err
	}
	if config.CollectParameter && len(invocation.Args()[1].([]interface{})) > 0 {
		span.Tag(tracing.TagDBSqlParameters, argsToString(invocation.Args()[1].([]interface{})))
	}
	invocation.SetContext(span)
	return nil
}

func (n *StmtQueryInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
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
