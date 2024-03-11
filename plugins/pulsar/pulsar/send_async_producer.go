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
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	pulsarAsyncPrefix      = "Pulsar/"
	pulsarAsyncSuffix      = "/AsyncProducer"
	pulsarCallbackSuffix   = "/Producer/Callback"
	pulsarAsyncComponentID = 73
)

type SendAsyncInterceptor struct {
}

func (s *SendAsyncInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	nativeProducer := invocation.CallerInstance().(*nativepartitionProducer)
	topic := nativeProducer.options.Topic
	msg := invocation.Args()[1].(*ProducerMessage)
	lookup, err := nativeProducer.client.lookupService.Lookup(topic)
	if err != nil {
		return err
	}
	peer := lookup.PhysicalAddr.String()
	operationName := pulsarAsyncPrefix + topic + pulsarAsyncSuffix

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
		tracing.WithComponent(pulsarAsyncComponentID),
		tracing.WithTag(tracing.TagMQBroker, lookup.PhysicalAddr.String()),
		tracing.WithTag(tracing.TagMQTopic, nativeProducer.topic),
	)
	if err != nil {
		return err
	}

	continueSnapShot := tracing.CaptureContext()
	zuper := invocation.Args()[2].(func(id MessageID, message *ProducerMessage, err error))

	callbackFunc := func(id MessageID, message *ProducerMessage, err error) {
		defer tracing.CleanContext()
		tracing.ContinueContext(continueSnapShot)
		operationName = pulsarAsyncPrefix + topic + pulsarCallbackSuffix

		localSpan, localErr := tracing.CreateLocalSpan(operationName,
			tracing.WithComponent(pulsarAsyncComponentID),
			tracing.WithLayer(tracing.SpanLayerMQ),
			tracing.WithTag(tracing.TagMQTopic, nativeProducer.topic),
		)
		if localErr != nil {
			zuper(id, message, err)
			return
		}
		if err != nil {
			span.Error(err.Error())
		}
		localSpan.Tag(tracing.TagMQBroker, lookup.PhysicalAddr.String())
		localSpan.Tag(tracing.TagMQMsgID, id.String())

		zuper(id, message, err)
		localSpan.SetPeer(peer)
		localSpan.End()
	}

	span.SetPeer(peer)
	invocation.ChangeArg(2, callbackFunc)
	invocation.SetContext(span)
	return nil
}

func (s *SendAsyncInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	invocation.GetContext().(tracing.Span).End()
	return nil
}
