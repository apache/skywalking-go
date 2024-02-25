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

package pulsar

import "net/url"

//skywalking:native github.com/apache/pulsar-client-go/pulsar partitionProducer
type nativepartitionProducer struct {
	client  *nativeclient
	topic   string
	options *nativeProducerOptions
}

//skywalking:native github.com/apache/pulsar-client-go/pulsar client
type nativeclient struct {
	lookupService nativeLookupService
}

//skywalking:native github.com/apache/pulsar-client-go/pulsar ProducerOptions
type nativeProducerOptions struct {
	Topic string
}

//skywalking:native github.com/apache/pulsar-client-go/pulsar/internal LookupService
type nativeLookupService interface {
	Lookup(topic string) (*LookupResult, error)
}

type LookupResult struct {
	LogicalAddr  *url.URL
	PhysicalAddr *url.URL
}

//skywalking:native github.com/apache/pulsar-client-go/pulsar MessageID
type MessageID interface {
	String() string
}

//skywalking:native github.com/apache/pulsar-client-go/pulsar ProducerMessage
type ProducerMessage struct {
	Properties map[string]string
}

//skywalking:native github.com/apache/pulsar-client-go/pulsar consumer
type nativeconsumer struct {
	client  *nativeclient
	topic   string
	options *nativeConsumerOptions
}

//skywalking:native github.com/apache/pulsar-client-go/pulsar ConsumerOptions
type nativeConsumerOptions struct {
	Topic string
}
