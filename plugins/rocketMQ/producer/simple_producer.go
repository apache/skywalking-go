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

package producer

import (
	"strings"

	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	rmqSyncSendPrefix  = "RocketMQ/"
	rmqProducerSuffix  = "/Producer"
	semicolon          = ";"
	rmqSyncComponentID = 38
)

func SimpleProducerInterceptor(invocation operator.Invocation) error {
	defaultProducer := invocation.CallerInstance().(*nativedefaultProducer)
	peer := strings.Join(defaultProducer.client.GetNameSrv().AddrList(), semicolon)
	msgList := invocation.Args()[1].([]*primitive.Message)
	topic := msgList[0].Topic
	operationName := rmqSyncSendPrefix + topic + rmqProducerSuffix

	span, err := tracing.CreateExitSpan(operationName, peer, func(headerKey, headerValue string) error {
		for _, message := range msgList {
			message.WithProperty(headerKey, headerValue)
		}
		return nil
	},
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(rmqSyncComponentID),
		tracing.WithTag(tracing.TagMQTopic, topic),
	)
	if err != nil {
		return err
	}
	invocation.SetContext(span)
	return nil
}
