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
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	ConsumerComponentID = 145
)

type ConsumerInterceptor struct{}

func (a *ConsumerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	channel := invocation.CallerInstance().(*nativeChannel)
	peer := getPeerInfo(channel.connection)
	msg := invocation.Args()[6].(amqp091.Table)
	queue, consumer := invocation.Args()[0].(string), invocation.Args()[1].(string)
	operationName := "Amqp/" + queue + "/" + consumer + "/Consumer"

	span, err := tracing.CreateEntrySpan(operationName, func(headerKey string) (string, error) {
		if msg[headerKey] != nil {
			return msg[headerKey].(string), nil
		}
		return "", nil
	},
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(ConsumerComponentID),
		tracing.WithTag(tracing.TagMQBroker, peer),
		tracing.WithTag(tracing.TagMQQueue, queue),
		tracing.WithTag(tracing.TagMQConsumer, consumer),
	)
	if err != nil {
		return err
	}
	span.SetPeer(peer)
	invocation.SetContext(span)
	return nil
}

func (a *ConsumerInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	span := invocation.GetContext().(tracing.Span)
	if err, ok := results[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
