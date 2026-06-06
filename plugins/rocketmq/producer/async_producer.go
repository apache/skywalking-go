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
	rmqASyncSuffix        = "/AsyncProducer"
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
	operationName := rmqASyncSendPrefix + topic + rmqASyncSuffix

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
	// enhance async callback method: the agent part is fully isolated inside
	// traceAsyncSendCallback (see its doc), the user callback runs after it
	callbackFunc := func(ctx context.Context, sendResult *primitive.SendResult, err error) {
		traceAsyncSendCallback(continueSnapShot, topic, peer, sendResult, err, func(brokerName string) string {
			return defaultProducer.client.GetNameSrv().FindBrokerAddrByName(brokerName)
		})
		zuper(ctx, sendResult, err)
	}

	span.SetPeer(peer)
	invocation.ChangeArg(1, callbackFunc)
	invocation.SetContext(span)
	return nil
}

// traceAsyncSendCallback records the async send result on a NEW local span -
// never on the exit span, already ended by AfterInvoke. It runs on an SDK
// goroutine without framework recover, so the agent logic is fully wrapped in
// its own recover; the user callback runs outside, never swallowed.
func traceAsyncSendCallback(snapshot tracing.ContextSnapshot, topic, peer string,
	sendResult *primitive.SendResult, sendErr error, brokerAddr func(brokerName string) string) {
	defer tracing.CleanContext()
	defer func() {
		// no logging channel exists on this goroutine, drop on purpose
		_ = recover()
	}()
	tracing.ContinueContext(snapshot)

	localSpan, err := tracing.CreateLocalSpan(rmqASyncSendPrefix+topic+rmqCallbackSuffix,
		tracing.WithComponent(rmqASyncComponentID),
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithTag(tracing.TagMQTopic, topic),
	)
	if err != nil {
		return
	}
	if sendErr != nil {
		localSpan.Error(sendErr.Error())
	}
	if sendResult != nil { // nil when the send failed
		localSpan.Tag(tracing.TagMQStatus, SendStatusStr(sendResult.Status))
		if sendResult.MessageQueue != nil {
			localSpan.Tag(tracing.TagMQQueue, fmt.Sprintf("%d", sendResult.MessageQueue.QueueId))
			localSpan.Tag(tracing.TagMQBroker, brokerAddr(sendResult.MessageQueue.BrokerName))
		}
		localSpan.Tag(tracing.TagMQMsgID, sendResult.MsgID)
		localSpan.Tag(aSyncTagMQOffsetMsgID, sendResult.OffsetMsgID)
	}
	localSpan.SetPeer(peer)
	localSpan.End()
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
