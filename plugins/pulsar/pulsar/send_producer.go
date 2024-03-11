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

package pulsar

import (
	"github.com/apache/pulsar-client-go/pulsar"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	pulsarSyncPrefix      = "Pulsar/"
	pulsarSyncSuffix      = "/Producer"
	pulsarSyncComponentID = 73
)

type SendInterceptor struct {
}

func (s *SendInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	defaultProducer := invocation.CallerInstance().(*nativepartitionProducer)
	topic := defaultProducer.options.Topic
	msg := invocation.Args()[1].(*pulsar.ProducerMessage)
	lookup, err := defaultProducer.client.lookupService.Lookup(topic)
	if err != nil {
		return err
	}
	peer := lookup.PhysicalAddr.String()
	operationName := pulsarSyncPrefix + topic + pulsarSyncSuffix

	span, err := tracing.CreateExitSpan(operationName, peer, func(headerKey, headerValue string) error {
		if msg.Properties == nil {
			msg.Properties = map[string]string{
				headerKey: headerValue,
			}
			return nil
		}
		msg.Properties[headerKey] = headerValue
		return nil
	},
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(pulsarSyncComponentID),
		tracing.WithTag(tracing.TagMQBroker, lookup.PhysicalAddr.String()),
		tracing.WithTag(tracing.TagMQTopic, defaultProducer.topic),
	)
	if err != nil {
		return err
	}

	invocation.SetContext(span)
	return nil
}

func (s *SendInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if err, ok := result[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	if msgRet, ok := result[0].(pulsar.MessageID); ok && msgRet != nil {
		span.Tag(tracing.TagMQMsgID, msgRet.String())
	}
	span.End()
	return nil
}
