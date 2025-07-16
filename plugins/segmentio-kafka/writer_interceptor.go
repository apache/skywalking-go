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

package segmentiokafka

import (
	"context"

	"github.com/segmentio/kafka-go"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const (
	kafkaWriterPrefix      = "Kafka/"
	kafkaWriterSuffix      = "/Producer"
	kafkaWriterComponentID = 40
)

var internalReporterContextKey = context.Background()

type WriterInterceptor struct {
}

func (w *WriterInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	writer := invocation.CallerInstance().(*kafka.Writer)
	addr, topic := writer.Addr.String(), writer.Topic
	ctx := invocation.Args()[0].(context.Context)
	if internal, ok := ctx.Value(internalReporterContextKey).(bool); ok && internal {
		return nil
	}
	messageList := invocation.Args()[1].([]kafka.Message)
	operationName := kafkaWriterPrefix + topic + kafkaWriterSuffix

	span, err := tracing.CreateExitSpan(operationName, addr, func(headerKey, headerValue string) error {
		for idx := range messageList {
			if len(messageList[idx].Headers) == 0 {
				messageList[idx].Headers = []kafka.Header{
					{Key: headerKey, Value: []byte(headerValue)},
				}
			} else {
				messageList[idx].Headers = append(messageList[idx].Headers,
					kafka.Header{Key: headerKey, Value: []byte(headerValue)})
			}
		}
		return nil
	},
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(kafkaWriterComponentID),
		tracing.WithTag(tracing.TagMQBroker, addr),
		tracing.WithTag(tracing.TagMQTopic, topic),
	)
	if err != nil {
		return err
	}

	span.SetPeer(addr)
	invocation.SetContext(span)
	return nil
}

func (w *WriterInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if err, ok := result[0].(error); ok && err != nil {
		span.Tag(tracing.TagMQStatus, err.Error())
		span.Error(err.Error())
	}
	span.End()
	return nil
}
