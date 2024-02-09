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
	"fmt"
	"github.com/rabbitmq/amqp091-go"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	ConsumerComponentID = 145
)

type ConsumerInterceptor struct{}

func (c *ConsumerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (c *ConsumerInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	deliveries := <-results[0].(<-chan Delivery)
	channel := invocation.CallerInstance().(*nativeChannel)
	peer := getPeerInfo(channel.connection)
	args := invocation.Args()[6].(amqp091.Table)
	queue, consumerTag := invocation.Args()[0].(string), invocation.Args()[1].(string)
	if consumerTag == "" {
		consumerTag = deliveries.ConsumerTag
	}
	operationName := "Amqp/" + queue + "/" + consumerTag + "/Consumer"

	span, err := tracing.CreateEntrySpan(operationName, func(headerKey string) (string, error) {
		return deliveries.Headers[headerKey].(string), nil
	}, tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(ConsumerComponentID),
		tracing.WithTag(tracing.TagMQBroker, peer),
		tracing.WithTag(tracing.TagMQQueue, queue),
		tracing.WithTag(tracing.TagMQConsumerTag, consumerTag),
		tracing.WithTag(tracing.TagMQCorrelationId, deliveries.CorrelationId),
		tracing.WithTag(tracing.TagMQReplyTo, deliveries.ReplyTo),
		tracing.WithTag(tracing.TagMQArgs, fmt.Sprintf("%v", args)),
	)
	if err != nil {
		return err
	}
	span.SetPeer(peer)
	if err, ok := results[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
