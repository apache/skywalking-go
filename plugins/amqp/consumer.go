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

package amqp

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
)

type ConsumersSendInterceptor struct {
}

func (c *ConsumersSendInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (c *ConsumersSendInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	return GeneralConsumersSendAfterInvoke(invocation, results...)
}

type ConsumerInterceptor struct {
}

func (c *ConsumerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return GeneralConsumerBeforeInvoke(invocation)
}

func (c *ConsumerInterceptor) AfterInvoke(operator.Invocation, ...interface{}) error {
	return nil
}

type ConsumersCloseInterceptor struct {
}

func (c *ConsumersCloseInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return GeneralConsumerCloseBeforeInvoke(invocation)
}

func (c *ConsumersCloseInterceptor) AfterInvoke(operator.Invocation, ...interface{}) error {
	return nil
}
