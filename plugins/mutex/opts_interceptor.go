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
	"github.com/apache/skywalking-go/plugins/core/operator"
)

type LockInterceptor struct {
}

func (h *LockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return LockBeforeInvoke(invocation, "Mutex/Lock")
}

func (h *LockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return LockAfterInvoke(invocation)
}

type TryLockInterceptor struct {
}

func (h *TryLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return TryLockBeforeInvoke(invocation, "Mutex/TryLock")
}

func (h *TryLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return TryLockAfterInvoke(invocation)
}

type UnLockInterceptor struct {
}

func (h *UnLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return UnlockBeforeInvoke(invocation, "Mutex/UnLock")
}

func (h *UnLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return UnlockAfterInvoke(invocation)
}

type RWRLockInterceptor struct {
}

func (h *RWRLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return LockBeforeInvoke(invocation, "RWMutex/RLock")
}

func (h *RWRLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return LockAfterInvoke(invocation)
}

type RWRTryLockInterceptor struct {
}

func (h *RWRTryLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return TryLockBeforeInvoke(invocation, "RWMutex/RTryLock")
}

func (h *RWRTryLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return TryLockAfterInvoke(invocation)
}

type RWRUnLockInterceptor struct {
}

func (h *RWRUnLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return UnlockBeforeInvoke(invocation, "RWMutex/RUnLock")
}

func (h *RWRUnLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return UnlockAfterInvoke(invocation)
}

type RWLockInterceptor struct {
}

func (h *RWLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return LockBeforeInvoke(invocation, "RWMutex/Lock")
}

func (h *RWLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return LockAfterInvoke(invocation)
}

type RWTryLockInterceptor struct {
}

func (h *RWTryLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return TryLockBeforeInvoke(invocation, "RWMutex/TryLock")
}

func (h *RWTryLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return TryLockAfterInvoke(invocation)
}

type RWUnLockInterceptor struct {
}

func (h *RWUnLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return UnlockBeforeInvoke(invocation, "RWMutex/UnLock")
}

func (h *RWUnLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return UnlockAfterInvoke(invocation)
}

type WaitGroupAddInterceptor struct {
}

func (h *WaitGroupAddInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	_, err := BaseBeforeInvoke(invocation, "WaitGroup/Add")
	return err
}

func (h *WaitGroupAddInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return BaseAfterInvoke(invocation)
}

type WaitGroupDoneInterceptor struct {
}

func (h *WaitGroupDoneInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	_, err := BaseBeforeInvoke(invocation, "WaitGroup/Done")
	return err
}

func (h *WaitGroupDoneInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return BaseAfterInvoke(invocation)
}

type WaitGroupWaitInterceptor struct {
}

func (h *WaitGroupWaitInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	_, err := BaseBeforeInvoke(invocation, "WaitGroup/Wait")
	return err
}

func (h *WaitGroupWaitInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return BaseAfterInvoke(invocation)
}
