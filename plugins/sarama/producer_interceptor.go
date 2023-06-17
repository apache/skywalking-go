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
	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"strings"

	"github.com/Shopify/sarama"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type AsyncProducerInterceptor struct {
}

// BeforeInvoke would be called before the target method invocation.
func (p *AsyncProducerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	client, ok := invocation.Args()[0].(sarama.Client)
	if !ok {
		return fmt.Errorf("sarama :skyWalking cannot create producer interceptor for client not match Client interface: %T", client)
	}
	conf := client.Config()
	var brokers []string
	for _, s := range client.Brokers() {
		brokers = append(brokers, s.Addr())
	}
	conf.Producer.Interceptors = append(conf.Producer.Interceptors, &producerInterceptor{
		brokers: brokers,
	})
	err := conf.Validate()
	if err != nil {
		return fmt.Errorf("sarama :skyWalking validate producer interceptor config failed: %v", err)
	}
	return nil
}

// AfterInvoke would be called after the target method invocation.
func (p *AsyncProducerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}

type producerInterceptor struct {
	brokers []string
}

func (p *producerInterceptor) OnSend(msg *sarama.ProducerMessage) {
	if msg == nil {
		// Panic protection. Should be unreachable.
		return
	}

	// If trace info is already existed in msg header, msg must be instrumented
	// in `SendMessage()` or `SendMessages()` in SyncProducer
	for _, h := range msg.Headers {
		k := string(h.Key)
		if k == core.Header || k == core.HeaderCorrelation {
			return
		}
	}

	// If trace info is not existed in msg header, msg must be sent by AsyncProducer.
	// Start a new exit Span.
	s, err := tracing.CreateExitSpan(
		// operationName
		"Kafka/"+msg.Topic+"/Producer",

		// peer
		strings.Join(p.brokers, ","),

		// injector
		func(k, v string) error {
			h := sarama.RecordHeader{
				Key: []byte(k), Value: []byte(v),
			}
			msg.Headers = append(msg.Headers, h)
			return nil
		},

		// opts
		tracing.WithTag(tracing.TagMQBroker, strings.Join(p.brokers, ",")),
		tracing.WithTag(tracing.TagMQTopic, msg.Topic),
		tracing.WithLayer(tracing.SpanLayerMQ),
		tracing.WithComponent(componentID),
	)

	if err != nil {
		return
	}

	s.End()
}
