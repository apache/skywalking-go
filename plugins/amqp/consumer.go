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
	"github.com/rabbitmq/amqp091-go"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

type ConsumerInterceptor struct {
}

func (c *ConsumerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (c *ConsumerInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	queue, consumerTag, args := invocation.Args()[0].(string), invocation.Args()[1].(string),
		invocation.Args()[6].(amqp091.Table)
	return GeneralConsumerAfterInvoke(invocation, queue, consumerTag, args, results...)
}
