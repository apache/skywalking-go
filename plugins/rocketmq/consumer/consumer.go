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

package consumer

import (
	"strings"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	rmqConsumerComponentID = 39
	rmqConsumerPrefix      = "RocketMQ/"
	rmqConsumerSuffix      = "/Consumer"
	tagMQMsgID             = "mq.msg.id"
	tagMQOffsetMsgID       = "mq.offset.msg.id"
	semicolon              = ";"
)

type SwConsumerInterceptor struct {
}

func (c *SwConsumerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	pushConsumer := invocation.CallerInstance().(*nativepushConsumer)
	peer := strings.Join(pushConsumer.client.GetNameSrv().AddrList(), semicolon)
	subMsgs := invocation.Args()[1].([]*primitive.MessageExt)
	if len(subMsgs) == 0 {
		return nil
	}
	topic, addr := subMsgs[0].Topic, subMsgs[0].StoreHost
	operationName := rmqConsumerPrefix + topic + rmqConsumerSuffix

	var (
		span tracing.Span
		err  error
	)
	for _, msg := range subMsgs {
		span, err = tracing.CreateEntrySpan(operationName, func(headerKey string) (string, error) {
			return msg.GetProperty(headerKey), nil
		},
			tracing.WithLayer(tracing.SpanLayerMQ),
			tracing.WithComponent(rmqConsumerComponentID),
			tracing.WithTag(tracing.TagMQTopic, topic),
			tracing.WithTag(tagMQMsgID, msg.MsgId),
			tracing.WithTag(tagMQOffsetMsgID, msg.OffsetMsgId),
		)
		if err != nil {
			return err
		}
	}
	span.Tag(tracing.TagMQBroker, addr)
	span.SetPeer(peer)
	invocation.SetContext(span)
	return nil
}

func (c *SwConsumerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if err, ok := result[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	if consumeRet, ok := result[0].(consumer.ConsumeResult); ok {
		span.Tag(tracing.TagMQStatus, SwConsumerStatusStr(consumeRet))
		if consumer.ConsumeSuccess != consumeRet {
			span.Error()
		}
	}
	span.End()
	return nil
}
