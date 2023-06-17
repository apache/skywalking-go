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
	"fmt"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/operator"

	"github.com/Shopify/sarama"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ConsumerInterceptor struct {
}

// BeforeInvoke would be called before the target method invocation.
func (c *ConsumerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	client, ok := invocation.Args()[0].(sarama.Client)
	if !ok {
		return fmt.Errorf("sarama :skyWalking cannot create consumer interceptor for client not match Client interface: %T", client)
	}
	conf := client.Config()
	var brokers []string
	for _, s := range client.Brokers() {
		brokers = append(brokers, s.Addr())
	}
	conf.Consumer.Interceptors = append(conf.Consumer.Interceptors, &consumerInterceptor{
		brokers: brokers,
	})
	err := conf.Validate()
	if err != nil {
		return fmt.Errorf("sarama :skyWalking validate consumer interceptor config failed: %v", err)
	}
	return nil
}

// AfterInvoke would be called after the target method invocation.
func (c *ConsumerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}

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
		tracing.WithComponent(5015),
	)

	if err != nil {
		return
	}

	s.End()
}
