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
	amqpConsumerPrefix  = "AMQP/"
	amqpConsumerSuffix  = "/Consumer"
	tagMQConsumerTag    = "mq.consumer_tag"
	tagMQReplyTo        = "mq.reply_to"
	tagMQCorrelationID  = "mq.correlation_id"
	tagMQArgs           = "mq.args"
)

func GeneralConsumerAfterInvoke(invocation operator.Invocation, queue, consumerTag string, args amqp091.Table, results ...interface{}) error {
	deliveries := <-results[0].(<-chan Delivery)
	if consumerTag == "" {
		consumerTag = deliveries.ConsumerTag
	}
	operationName := amqpConsumerPrefix + queue + "/" + consumerTag + amqpConsumerSuffix

	channel := invocation.CallerInstance().(*nativeChannel)
	peer := getPeerInfo(channel.connection)

	span, err := tracing.CreateEntrySpan(operationName, func(headerKey string) (string, error) {
		return deliveries.Headers[headerKey].(string), nil
	}, tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(ConsumerComponentID),
		tracing.WithTag(tracing.TagMQBroker, peer),
		tracing.WithTag(tracing.TagMQQueue, queue),
		tracing.WithTag(tracing.TagMQMsgID, deliveries.MessageId),
		tracing.WithTag(tagMQConsumerTag, consumerTag),
		tracing.WithTag(tagMQCorrelationID, deliveries.CorrelationId),
		tracing.WithTag(tagMQReplyTo, deliveries.ReplyTo),
		tracing.WithTag(tagMQArgs, fmt.Sprintf("%v", args)),
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
