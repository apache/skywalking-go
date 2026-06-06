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
	span, err := createConsumerEntrySpan(subMsgs, peer)
	if err != nil || span == nil {
		return err
	}
	invocation.SetContext(span)
	return nil
}

// createConsumerEntrySpan creates ONE entry span for the whole batch from the
// first message and attaches every remaining message as an extra segment
// reference, mirroring the Java agent. One span per message must be avoided:
// the reuse rule would hand back the same span N times while AfterInvoke
// calls End only once, so the span would never be reported.
func createConsumerEntrySpan(subMsgs []*primitive.MessageExt, peer string) (tracing.Span, error) {
	if len(subMsgs) == 0 {
		return nil, nil
	}
	first := subMsgs[0]
	topic := first.Topic
	msgIDs := make([]string, 0, len(subMsgs))
	offsetMsgIDs := make([]string, 0, len(subMsgs))
	for _, msg := range subMsgs {
		msgIDs = append(msgIDs, msg.MsgId)
		offsetMsgIDs = append(offsetMsgIDs, msg.OffsetMsgId)
	}

	span, err := tracing.CreateEntrySpan(rmqConsumerPrefix+topic+rmqConsumerSuffix, func(headerKey string) (string, error) {
		return first.GetProperty(headerKey), nil
	},
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(rmqConsumerComponentID),
		tracing.WithTag(tracing.TagMQTopic, topic),
		tracing.WithTag(tagMQMsgID, strings.Join(msgIDs, semicolon)),
		tracing.WithTag(tagMQOffsetMsgID, strings.Join(offsetMsgIDs, semicolon)),
	)
	if err != nil {
		return nil, err
	}
	for _, msg := range subMsgs[1:] {
		extractMsg := msg
		// a broken header on a single message must not lose the batch span,
		// so the error is intentionally ignored
		_ = tracing.ExtractContext(func(headerKey string) (string, error) {
			return extractMsg.GetProperty(headerKey), nil
		})
	}
	span.Tag(tracing.TagMQBroker, first.StoreHost)
	span.SetPeer(peer)
	return span, nil
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
