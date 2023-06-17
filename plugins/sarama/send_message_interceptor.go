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

package sarama

import (
	"strings"

	"github.com/Shopify/sarama"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type SendMessageInterceptor struct {
}

type SendMessagesInterceptor struct {
}

// BeforeInvoke would be called before the target method invocation.
func (s *SendMessageInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	syncProducerEnhanced, ok := invocation.CallerInstance().(operator.EnhancedInstance)
	if !ok {
		return nil
	}

	brokers, ok := syncProducerEnhanced.GetSkyWalkingDynamicField().([]string)
	if !ok {
		return nil
	}

	msg, ok := invocation.Args()[0].(*sarama.ProducerMessage)
	if !ok {
		return nil
	}

	// If trace info is not existed in msg header, msg must be sent by AsyncProducer.
	// Start a new exit Span.
	span, err := tracing.CreateExitSpan(
		// operationName
		"Kafka/"+msg.Topic+"/Producer",

		// peer
		strings.Join(brokers, ","),

		// injector
		func(k, v string) error {
			h := sarama.RecordHeader{
				Key: []byte(k), Value: []byte(v),
			}
			msg.Headers = append(msg.Headers, h)
			return nil
		},

		// opts
		tracing.WithTag(tracing.TagMQBroker, strings.Join(brokers, ",")),
		tracing.WithTag(tracing.TagMQTopic, msg.Topic),
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(componentID),
	)

	if err != nil {
		return nil
	}

	invocation.SetContext(span)

	return nil
}

// AfterInvoke would be called after the target method invocation.
func (s *SendMessageInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}

	invocation.GetContext().(tracing.Span).End()
	return nil
}

// BeforeInvoke would be called before the target method invocation.
func (s *SendMessagesInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	syncProducerEnhanced, ok := invocation.CallerInstance().(operator.EnhancedInstance)
	if !ok {
		return nil
	}

	brokers, ok := syncProducerEnhanced.GetSkyWalkingDynamicField().([]string)
	if !ok {
		return nil
	}

	msgs, ok := invocation.Args()[0].([]*sarama.ProducerMessage)
	if !ok {
		return nil
	}

	topics := make([]string, 0, len(msgs))
	for i := range msgs {
		topics = append(topics, msgs[i].Topic)
	}

	// If trace info is not existed in msg header, msg must be sent by AsyncProducer.
	// Start a new exit Span.
	span, err := tracing.CreateExitSpan(
		// operationName
		"Kafka/batch/Producer",

		// peer
		strings.Join(brokers, ","),

		// injector
		func(k, v string) error {
			h := sarama.RecordHeader{
				Key: []byte(k), Value: []byte(v),
			}
			for i := range msgs {
				msgs[i].Headers = append(msgs[i].Headers, h)
			}
			return nil
		},

		// opts
		tracing.WithTag(tracing.TagMQBroker, strings.Join(brokers, ",")),
		tracing.WithTag(tracing.TagMQTopic, strings.Join(topics, ",")),
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(componentID),
	)

	if err != nil {
		return nil
	}

	invocation.SetContext(span)

	return nil
}

// AfterInvoke would be called after the target method invocation.
func (s *SendMessagesInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}

	invocation.GetContext().(tracing.Span).End()
	return nil
}
