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
	amqpSendPrefix      = "AMQP"
	amqpSendSuffix      = "/Producer"
	delimiter           = "/"
	ProducerComponentID = 144
	tagMQExchange       = "mq.exchange"
	tagMQRoutingKey     = "mq.routing_key"
)

// The publish methods call each other, and the order changed in amqp091-go v1.10.0:
// PublishWithDeferredConfirmWithContext was the common implementation before, and
// PublishWithDeferredConfirm is the common one since, reached by Publish and
// PublishWithContext. Both are intercepted, and producerActiveKey keeps the nested
// call from creating a second span for the same message.
const producerActiveKey = "amqp-producer-active"

// ProducerInterceptor intercepts (*Channel).PublishWithDeferredConfirmWithContext.
type ProducerInterceptor struct{}

func (p *ProducerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return beforePublish(invocation, 1)
}

func (p *ProducerInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	return afterPublish(invocation, results...)
}

// ProducerWithoutContextInterceptor intercepts (*Channel).PublishWithDeferredConfirm.
type ProducerWithoutContextInterceptor struct{}

func (p *ProducerWithoutContextInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return beforePublish(invocation, 0)
}

func (p *ProducerWithoutContextInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	return afterPublish(invocation, results...)
}

// beforePublish creates the producer span. offset is the index of the exchange
// argument, which is followed by the routing key, mandatory, immediate, and the message.
func beforePublish(invocation operator.Invocation, offset int) error {
	if tracing.GetRuntimeContextValue(producerActiveKey) != nil {
		return nil
	}
	args := invocation.Args()
	publishing, ok := args[offset+4].(amqp091.Publishing)
	if !ok {
		return nil
	}
	exchange, _ := args[offset].(string)
	routingKey, _ := args[offset+1].(string)
	operationName := amqpSendPrefix
	if exchange != "" {
		operationName += delimiter + exchange
	}
	if routingKey != "" {
		operationName += delimiter + routingKey
	}
	operationName += amqpSendSuffix

	channel := invocation.CallerInstance().(*nativeChannel)
	peer := getPeerInfo(channel.connection)
	span, err := tracing.CreateExitSpan(operationName, peer, func(headerKey, headerValue string) error {
		if publishing.Headers == nil {
			publishing.Headers = amqp091.Table{}
		}
		publishing.Headers[headerKey] = headerValue
		return nil
	}, tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(ProducerComponentID),
		tracing.WithTag(tracing.TagMQBroker, peer),
		tracing.WithTag(tagMQExchange, exchange),
		tracing.WithTag(tagMQRoutingKey, routingKey),
	)
	if err != nil {
		return err
	}
	// The message is passed by value, so a header table created by the injector
	// only reaches the broker through the changed argument.
	invocation.ChangeArg(offset+4, publishing)
	tracing.SetRuntimeContextValue(producerActiveKey, true)
	invocation.SetContext(span)
	return nil
}

func afterPublish(invocation operator.Invocation, results ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	tracing.SetRuntimeContextValue(producerActiveKey, nil)
	span := invocation.GetContext().(tracing.Span)
	if err, ok := results[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
