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
	"context"
	"fmt"
	"strings"

	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	rmqASyncSendPrefix    = "RocketMQ/"
	rmqCallbackSuffix     = "/Producer/Callback"
	rmqASyncComponentID   = 38
	aSyncSemicolon        = ";"
	aSyncTagMQOffsetMsgID = "mq.offset.msg.id"
)

type SendASyncInterceptor struct {
}

func (sa *SendASyncInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	defaultProducer := invocation.CallerInstance().(*nativedefaultProducer)
	peer := strings.Join(defaultProducer.client.GetNameSrv().AddrList(), aSyncSemicolon)
	msgList := invocation.Args()[2].([]*primitive.Message)
	topic := msgList[0].Topic
	operationName := rmqASyncSendPrefix + topic + rmqCallbackSuffix

	span, err := tracing.CreateExitSpan(operationName, peer, func(headerKey, headerValue string) error {
		for _, message := range msgList {
			message.WithProperty(headerKey, headerValue)
		}
		return nil
	},
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(rmqASyncComponentID),
		tracing.WithTag(tracing.TagMQTopic, topic),
	)
	if err != nil {
		return err
	}

	continueSnapShot := tracing.CaptureContext()
	zuper := invocation.Args()[1].(func(ctx context.Context, result *primitive.SendResult, err error))
	
	// enhanced async callback method
	callbackFunc := func(ctx context.Context, sendResult *primitive.SendResult, err error) {
		defer tracing.CleanContext()
		tracing.ContinueContext(continueSnapShot)
		operationName = rmqASyncSendPrefix + topic + rmqCallbackSuffix

		localSpan, localErr := tracing.CreateLocalSpan(operationName,
			tracing.WithComponent(rmqASyncComponentID),
			tracing.WithLayer(tracing.SpanLayerMQ),
			tracing.WithTag(tracing.TagMQTopic, topic),
		)
		if localErr != nil {
			zuper(ctx, sendResult, err)
			return
		}
		if err != nil {
			span.Error(err.Error())
		}
		localSpan.Tag(tracing.TagMQStatus, SendStatusStr(sendResult.Status))
		localSpan.Tag(tracing.TagMQQueue, fmt.Sprintf("%d", sendResult.MessageQueue.QueueId))
		localSpan.Tag(tracing.TagMQBroker, defaultProducer.client.GetNameSrv().
			FindBrokerAddrByName(sendResult.MessageQueue.BrokerName))
		localSpan.Tag(tracing.TagMQMsgID, sendResult.MsgID)
		localSpan.Tag(aSyncTagMQOffsetMsgID, sendResult.OffsetMsgID)

		zuper(ctx, sendResult, err)
		localSpan.SetPeer(peer)
		localSpan.End()
	}

	span.SetPeer(peer)
	invocation.ChangeArg(1, callbackFunc)
	invocation.SetContext(span)
	return nil
}

func (sa *SendASyncInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if err, ok := result[0].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
