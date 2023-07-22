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

package mutex

import (
	"github.com/apache/skywalking-go/plugins/core/metrics"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

var componentID int32 = 5016

var lockingIsExecutingKey = "lockingIsExecuting"
var lockingSamplerObjectKey = "lockingSamplerObject"

var acquisitionLockTimer, lockUseTimeTimer metrics.Timer
var tryLockErrorCounter metrics.Counter

const (
	tryLockResult        = "lock.result"
	tryLockResultSuccess = "true"
	tryLockResultFailure = "false"
)

//nolint
//skywalking:init
func registerMetrics() {
	acquisitionLockTimer = metrics.NewTimer("instance_golang_mutex_acquisition")
	lockUseTimeTimer = metrics.NewTimer("instance_golang_mutex_use_time")
	tryLockErrorCounter = metrics.NewCounter("instance_golang_mutex_try_lock_error")
}

type invocationContext struct {
	acquisitionLock metrics.TimerSample
	lockUseTime     metrics.TimerSample
	span            tracing.Span
}

//nolint
func BaseBeforeInvoke(invocation operator.Invocation, name string) (*invocationContext, error) {
	// must have tracing context
	span := tracing.ActiveSpan()
	if span == nil {
		return nil, nil
	}
	// ignore if already in locking span(avoid recursive call)
	if isLocking := tracing.GetRuntimeContextValue(lockingIsExecutingKey); isLocking != nil {
		return nil, nil
	}
	s, err := tracing.CreateLocalSpan(name, tracing.WithComponent(componentID))
	if err != nil {
		return nil, err
	}
	tracing.SetRuntimeContextValue(lockingIsExecutingKey, true)
	ctx := &invocationContext{span: s}
	invocation.SetContext(ctx)
	return ctx, nil
}

func LockBeforeInvoke(invocation operator.Invocation, name string) error {
	ctx, err := BaseBeforeInvoke(invocation, name)
	if ctx != nil && acquisitionLockTimer != nil && lockUseTimeTimer != nil {
		ctx.acquisitionLock = acquisitionLockTimer.Start()
		ctx.lockUseTime = lockUseTimeTimer.Start()
	}
	return err
}

func LockAfterInvoke(invocation operator.Invocation) error {
	ctx, _, err := baseTracingAfterInvoke(invocation)
	if ctx != nil && ctx.acquisitionLock != nil {
		// record acquisition lock time
		ctx.acquisitionLock.Stop()
		// save the locking sampler object to runtime context
		tracing.SetRuntimeContextValue(lockingSamplerObjectKey, ctx.lockUseTime)
	}
	return err
}

func TryLockBeforeInvoke(invocation operator.Invocation, name string) error {
	ctx, err := BaseBeforeInvoke(invocation, name)
	if ctx != nil && acquisitionLockTimer != nil && lockUseTimeTimer != nil {
		ctx.acquisitionLock = acquisitionLockTimer.Start()
		ctx.lockUseTime = lockUseTimeTimer.Start()
	}
	return err
}

func TryLockAfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	ctx, _, err := baseTracingAfterInvoke(invocation)
	if success, ok := result[0].(bool); ok && ctx != nil {
		tagTryLockResult(ctx.span, success)
		if !success {
			if tryLockErrorCounter != nil {
				tryLockErrorCounter.Inc(1)
			}
			return err
		}
		if ctx.acquisitionLock != nil {
			// record acquisition lock time
			ctx.acquisitionLock.Stop()
			// save the locking sampler object to runtime context
			tracing.SetRuntimeContextValue(lockingSamplerObjectKey, ctx.lockUseTime)
		}
	}
	return err
}

func UnlockBeforeInvoke(invocation operator.Invocation, name string) error {
	_, err := BaseBeforeInvoke(invocation, name)
	return err
}

func UnlockAfterInvoke(invocation operator.Invocation) error {
	_, _, err := baseTracingAfterInvoke(invocation)
	if useTimeSampler, ok := tracing.GetRuntimeContextValue(lockingSamplerObjectKey).(metrics.TimerSample); ok && useTimeSampler != nil {
		// record lock use time
		useTimeSampler.Stop()
	}
	return err
}

func BaseAfterInvoke(invocation operator.Invocation) error {
	_, _, err := baseTracingAfterInvoke(invocation)
	return err
}

//nolint
func baseTracingAfterInvoke(invocation operator.Invocation) (*invocationContext, tracing.Span, error) {
	if invocation.GetContext() == nil {
		return nil, nil, nil
	}
	ctx := invocation.GetContext().(*invocationContext)
	ctx.span.End()
	tracing.SetRuntimeContextValue(lockingIsExecutingKey, nil)
	return ctx, ctx.span, nil
}

func tagTryLockResult(span tracing.Span, success bool) {
	if success {
		span.Tag(tryLockResult, tryLockResultSuccess)
		return
	}
	span.Tag(tryLockResult, tryLockResultFailure)
}
