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
	"gorm.io/gorm"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// executeFrameKey is the runtime context key of the innermost
// (*processor).Execute call running on the current goroutine.
const executeFrameKey = "skywalking-gorm-execute-frame"

// executeFrame records the span that the start callback created during one
// Execute call, and the statement that holds it.
type executeFrame struct {
	span tracing.Span
	db   *gorm.DB
}

type executeContext struct {
	frame  *executeFrame
	parent interface{}
}

// ExecuteInterceptor ends a span that the end callback left open.
// gorm runs the callback chain without defer, so a panic in a callback or a
// hook (AfterFind, AfterCreate, ...) skips the end callback, and the open span
// would keep the whole segment from being reported. The agent defers the
// after-hook of (*processor).Execute, so it also runs while such a panic
// unwinds. It does not recover the panic: the application still sees it.
type ExecuteInterceptor struct {
}

func (e *ExecuteInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	frame := &executeFrame{}
	invocation.SetContext(&executeContext{
		frame:  frame,
		parent: tracing.GetRuntimeContextValue(executeFrameKey),
	})
	tracing.SetRuntimeContextValue(executeFrameKey, frame)
	return nil
}

func (e *ExecuteInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	ctx, ok := invocation.GetContext().(*executeContext)
	if !ok || ctx == nil {
		return nil
	}
	tracing.SetRuntimeContextValue(executeFrameKey, ctx.parent)

	frame := ctx.frame
	if frame.span == nil || frame.db == nil {
		return nil
	}
	// only the span that this call started, and only when the end callback
	// did not consume it: a statement shared across goroutines (unsupported
	// by gorm) can hold the span of another goroutine
	span := statementSpan(frame.db)
	if span == nil || span != frame.span {
		return nil
	}
	clearStatementSpan(frame.db)

	// Execute always returns the *gorm.DB, so a nil result means that it did
	// not return: a callback or hook panicked
	if ret, _ := results[0].(*gorm.DB); ret == nil {
		frame.span.Error("the gorm callbacks did not complete, a callback or hook panicked")
	}
	endSpan(frame.db, frame.span)
	return nil
}

// recordExecuteSpan hands a started span to the Execute call that runs the
// callbacks on this goroutine.
func recordExecuteSpan(db *gorm.DB, span tracing.Span) {
	if frame, ok := tracing.GetRuntimeContextValue(executeFrameKey).(*executeFrame); ok && frame != nil {
		frame.span = span
		frame.db = db
	}
}
