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
	pulsarReceivePrefix      = "Pulsar/"
	pulsarReceiveSuffix      = "/Consumer"
	pulsarReceiveComponentID = 74
)

type ReceiveInterceptor struct {
}

func (r *ReceiveInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (r *ReceiveInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	nativeConsumer := invocation.CallerInstance().(*nativeconsumer)
	topic := nativeConsumer.options.Topic
	lookup, err := nativeConsumer.client.lookupService.Lookup(topic)
	if err != nil {
		return err
	}
	message := result[0].(pulsar.Message)
	peer := lookup.LogicalAddr.String()
	operationName := pulsarReceivePrefix + topic + pulsarReceiveSuffix

	span, err := tracing.CreateEntrySpan(operationName, func(headerKey string) (string, error) {
		return message.Properties()[headerKey], nil
	},
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(pulsarReceiveComponentID),
		tracing.WithTag(tracing.TagMQBroker, lookup.PhysicalAddr.String()),
		tracing.WithTag(tracing.TagMQTopic, nativeConsumer.topic),
	)
	if err != nil {
		return err
	}

	if err, ok := result[1].(pulsar.Error); ok {
		span.Tag(tracing.TagMQStatus, err.Error())
		span.Error(err.Error())
	}
	span.SetPeer(peer)
	span.End()
	return nil
}
