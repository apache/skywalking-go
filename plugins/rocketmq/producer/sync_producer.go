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
	"fmt"

	"github.com/apache/rocketmq-client-go/v2/primitive"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	syncTagMQOffsetMsgID = "mq.offset.msg.id"
)

type SendSyncInterceptor struct {
}

func (s *SendSyncInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return GeneralProducerBeforeInvoke(invocation)
}

func (s *SendSyncInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	defaultProducer := invocation.CallerInstance().(*nativedefaultProducer)
	span := invocation.GetContext().(tracing.Span)
	if err, ok := result[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	if sendRet, ok := result[0].(*primitive.SendResult); ok && sendRet != nil {
		span.Tag(tracing.TagMQStatus, SendStatusStr(sendRet.Status))
		span.Tag(tracing.TagMQQueue, fmt.Sprintf("%d", sendRet.MessageQueue.QueueId))
		span.Tag(tracing.TagMQBroker, defaultProducer.client.GetNameSrv().
			FindBrokerAddrByName(sendRet.MessageQueue.BrokerName))
		span.Tag(tracing.TagMQMsgID, sendRet.MsgID)
		span.Tag(syncTagMQOffsetMsgID, sendRet.OffsetMsgID)
	}
	span.End()
	return nil
}
