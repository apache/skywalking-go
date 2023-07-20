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

var tryLockResult = "lock.result"

type LockInterceptor struct {
}

func (h *LockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "Mutex/Lock")
}

func (h *LockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type TryLockInterceptor struct {
}

func (h *TryLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "Mutex/TryLock")
}

func (h *TryLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvokeWithTag(invocation, tryLockResult, getTryResultTagValue(result))
}

type UnLockInterceptor struct {
}

func (h *UnLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "Mutex/UnLock")
}

func (h *UnLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type RWRLockInterceptor struct {
}

func (h *RWRLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "RWMutex/RLock")
}

func (h *RWRLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type RWRTryLockInterceptor struct {
}

func (h *RWRTryLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "RWMutex/RTryLock")
}

func (h *RWRTryLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvokeWithTag(invocation, tryLockResult, getTryResultTagValue(result))
}

type RWRUnLockInterceptor struct {
}

func (h *RWRUnLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "RWMutex/RUnLock")
}

func (h *RWRUnLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type RWLockInterceptor struct {
}

func (h *RWLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "RWMutex/Lock")
}

func (h *RWLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type RWTryLockInterceptor struct {
}

func (h *RWTryLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "RWMutex/TryLock")
}

func (h *RWTryLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvokeWithTag(invocation, tryLockResult, getTryResultTagValue(result))
}

type RWUnLockInterceptor struct {
}

func (h *RWUnLockInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "RWMutex/UnLock")
}

func (h *RWUnLockInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type WaitGroupAddInterceptor struct {
}

func (h *WaitGroupAddInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "WaitGroup/Add")
}

func (h *WaitGroupAddInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type WaitGroupDoneInterceptor struct {
}

func (h *WaitGroupDoneInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "WaitGroup/Done")
}

func (h *WaitGroupDoneInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

type WaitGroupWaitInterceptor struct {
}

func (h *WaitGroupWaitInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return BeforeInvoke(invocation, "WaitGroup/Wait")
}

func (h *WaitGroupWaitInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return AfterInvoke(invocation)
}

func getTryResultTagValue(result []interface{}) string {
	if b, ok := result[0].(bool); ok && b {
		return "true"
	}
	return "false"
}
