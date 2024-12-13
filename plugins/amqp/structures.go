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

package amqp

import (
	"io"
	"sync"
)

//skywalking:native github.com/rabbitmq/amqp091-go Channel
type nativeChannel struct {
	connection *nativeConnection
}

func (ch *nativeChannel) Ack(tag uint64, multiple bool) error {
	return nil
}
func (ch *nativeChannel) Nack(tag uint64, multiple bool, requeue bool) error {
	return nil
}
func (ch *nativeChannel) Reject(tag uint64, requeue bool) error {
	return nil
}

//skywalking:native github.com/rabbitmq/amqp091-go Delivery
type Delivery struct {
	Acknowledger  nativeAcknowledger
	Headers       Table
	MessageId     string //nolint
	ConsumerTag   string
	Exchange      string
	RoutingKey    string
	DeliveryTag   uint64
	CorrelationId string //nolint
	ReplyTo       string
}

type Table map[string]interface{}

//skywalking:native github.com/rabbitmq/amqp091-go Connection
type nativeConnection struct {
	conn io.ReadWriteCloser
}

//skywalking:native github.com/rabbitmq/amqp091-go Acknowledger
type nativeAcknowledger interface {
	Ack(tag uint64, multiple bool) error
	Nack(tag uint64, multiple bool, requeue bool) error
	Reject(tag uint64, requeue bool) error
}

//skywalking:native github.com/rabbitmq/amqp091-go consumers
type nativeConsumers struct {
	sync.Mutex
	chans map[string]chan *Delivery
}
