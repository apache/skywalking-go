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

type ProducerInterceptor struct{}

func (p *ProducerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	channel := invocation.CallerInstance().(*nativeChannel)
	peer := getPeerInfo(channel.connection)
	exchange, routingKey, operationName := invocation.Args()[1].(string), invocation.Args()[2].(string), amqpSendPrefix
	if exchange != "" {
		exchange = invocation.Args()[1].(string)
		operationName += delimiter + exchange
	}
	if routingKey != "" {
		routingKey = invocation.Args()[2].(string)
		operationName += delimiter + routingKey
	}
	publishing := invocation.Args()[5].(amqp091.Publishing)
	operationName += amqpSendSuffix

	span, err := tracing.CreateExitSpan(operationName, peer, func(headerKey, headerValue string) error {
		if publishing.Headers == nil {
			publishing.Headers = amqp091.Table{
				headerKey: headerValue,
			}
			return nil
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
	invocation.SetContext(span)
	return nil
}

func (p *ProducerInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if err, ok := results[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
