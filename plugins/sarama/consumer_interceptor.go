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

package sarama

import (
	"strings"

	"github.com/Shopify/sarama"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type consumerInterceptor struct {
	brokers []string
}

func (c *consumerInterceptor) OnConsume(msg *sarama.ConsumerMessage) {
	s, err := tracing.CreateEntrySpan(
		// operationName
		"Kafka/"+msg.Topic+"/Consumer",

		// extractor
		func(k string) (string, error) {
			// find SkyWalking header in msg.Headers
			for _, h := range msg.Headers {
				if string(h.Key) == k {
					return string(h.Value), nil
				}
			}
			return "", nil
		},
		// opts
		tracing.WithTag(tracing.TagMQBroker, strings.Join(c.brokers, ",")),
		tracing.WithTag(tracing.TagMQTopic, msg.Topic),
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(componentID),
	)

	if err != nil {
		return
	}

	s.End()
}
