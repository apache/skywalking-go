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
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
	"os"
	"strconv"
	"sync/atomic"
)

const (
	ConsumerComponentID  = 145
	amqpConsumerPrefix   = "AMQP/"
	amqpConsumerSuffix   = "/Consumer"
	tagMQConsumerTag     = "mq.consumer_tag"
	tagMQReplyTo         = "mq.reply_to"
	tagMQCorrelationID   = "mq.correlation_id"
	tagMQArgs            = "mq.args"
	consumerTagLengthMax = 0xFF
)

var consumerSeq uint64
var queueConsumerTagMapping = make(map[string]string)

func GeneralConsumersSendAfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if foundConsumer := results[0].(bool); !foundConsumer {
		return nil
	}
	consumerTag, _ := invocation.Args()[0].(string)
	delivery, _ := invocation.Args()[1].(*Delivery)
	operationName := amqpConsumerPrefix + queueConsumerTagMapping[consumerTag] + "/" + consumerTag + amqpConsumerSuffix
	channel, _ := delivery.Acknowledger.(*nativeChannel)
	peer := getPeerInfo(channel.connection)

	span, err := tracing.CreateEntrySpan(operationName, func(headerKey string) (string, error) {
		header, _ := delivery.Headers[headerKey].(string)
		return header, nil
	}, tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(ConsumerComponentID),
		tracing.WithTag(tracing.TagMQBroker, peer),
		tracing.WithTag(tracing.TagMQQueue, queueConsumerTagMapping[consumerTag]),
		tracing.WithTag(tracing.TagMQMsgID, delivery.MessageId),
		tracing.WithTag(tagMQConsumerTag, consumerTag),
		tracing.WithTag(tagMQCorrelationID, delivery.CorrelationId),
		tracing.WithTag(tagMQReplyTo, delivery.ReplyTo),
		tracing.WithTag(tagMQArgs, fmt.Sprintf("%v", delivery.Headers)),
	)
	if err != nil {
		return err
	}
	span.SetPeer(peer)
	span.End()
	return nil
}

func GeneralConsumerBeforeInvoke(invocation operator.Invocation) error {
	queue := invocation.Args()[0].(string)
	consumerTag := invocation.Args()[1].(string)
	if consumerTag == "" {
		consumerTag = uniqueConsumerTag()
	}
	queueConsumerTagMapping[consumerTag] = queue
	return nil
}

func GeneralConsumerCloseBeforeInvoke(invocation operator.Invocation) error {
	consumers, _ := invocation.CallerInstance().(*nativeConsumers)
	consumers.Lock()
	defer consumers.Unlock()
	for consumerTag := range consumers.chans {
		delete(queueConsumerTagMapping, consumerTag)
	}
	return nil
}

func uniqueConsumerTag() string {
	return commandNameBasedUniqueConsumerTag(os.Args[0])
}

func commandNameBasedUniqueConsumerTag(commandName string) string {
	tagPrefix := "ctag-"
	tagInfix := commandName
	tagSuffix := "-" + strconv.FormatUint(atomic.AddUint64(&consumerSeq, 1), 10)

	if len(tagPrefix)+len(tagInfix)+len(tagSuffix) > consumerTagLengthMax {
		tagInfix = "streadway/amqp"
	}

	return tagPrefix + tagInfix + tagSuffix
}
