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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	_ "github.com/apache/skywalking-go"
)

type testFunc func(RabbitClient) error

var (
	uri                    = "amqp://admin:123456@amqp-server:5672"
	queue1                 = "sw-queue-1"
	queue2                 = "sw-queue-2"
	body                   = "I love skywalking 3 thousand"
	consumerTag1           = "sw-consumer-1"
	consumerTag2           = "sw-consumer-2"
	consumerTrigger        = make(chan struct{})
	consumerWithCtxTrigger = make(chan struct{})
)

func main() {
	conn, err := amqp.Dial(uri)
	if err != nil {
		panic(err)
	}
	client, err := NewRabbitMQClient(conn)
	if err != nil {
		panic(err)
	}

	route := http.NewServeMux()
	route.HandleFunc("/execute", func(res http.ResponseWriter, req *http.Request) {
		tests := []struct {
			name string
			fn   testFunc
		}{
			{"testSimpleConsumer", testSimpleConsumer},
			{"testConsumerWithCtx", testConsumerWithCtx},
		}
		for _, test := range tests {
			fmt.Printf("excute test case: %s\n", test.name)
			if subErr := test.fn(client); subErr != nil {
				fmt.Printf("test case %s failed: %v", test.name, subErr)
			}
		}
		_, _ = res.Write([]byte("execute success"))
	})
	route.HandleFunc("/health", func(res http.ResponseWriter, req *http.Request) {
		_, _ = res.Write([]byte("ok"))
	})

	err = http.ListenAndServe(":8080", route)
	if err != nil {
		log.Fatalf("client start error: %v \n", err)
	}
	select {}
}

func testSimpleConsumer(client RabbitClient) error {
	producer(queue1, client)
	go consumer()
	consumerTrigger <- struct{}{}
	time.Sleep(time.Second)
	return nil
}

func testConsumerWithCtx(client RabbitClient) error {
	producer(queue2, client)
	go consumerWithContext()
	consumerWithCtxTrigger <- struct{}{}
	time.Sleep(time.Second)
	return nil
}

func producer(queue string, client RabbitClient) {
	client.CreateQueue(queue, true, false)
	if err := client.Send(context.Background(), "", queue, amqp.Publishing{
		ContentType:   "text/plain",
		Body:          []byte(body),
		Headers:       amqp.Table{},
		CorrelationId: "1",
		MessageId:     "2",
	}); err != nil {
		fmt.Println("Failed to Send msg, err: ", err)
	}
}

func consumer() {
	<-consumerTrigger
	consumeConn, err := amqp.Dial(uri)
	if err != nil {
		fmt.Println("Failed to Dial Consume, err: ", err)
	}
	consumeClient, err := NewRabbitMQClient(consumeConn)
	if err != nil {
		fmt.Println("Failed to Channel Consume, err: ", err)
	}
	msgs, err := consumeClient.Consume(queue1, consumerTag1, false)
	if err != nil {
		fmt.Println("Failed to Consume msg, err: ", err)
	}
	log.Printf("[Consumer] Waiting for messages.\n")
	for d := range msgs {
		log.Printf("Received a message: %s\n", string(d.Body))
		d.Ack(false)
	}
	err = consumeClient.Cancel(consumerTag1)
	if err != nil {
		fmt.Println("Failed to Cancel Consume, err: ", err)
	}
	err = consumeConn.Close()
	if err != nil {
		fmt.Println("Failed to Close Cancel, err: ", err)
	}
}

func consumerWithContext() {
	<-consumerWithCtxTrigger
	consumeConn, err := amqp.Dial(uri)
	if err != nil {
		fmt.Println("Failed to Dial ConsumerWithContext, err: ", err)
	}
	consumeClient, err := NewRabbitMQClient(consumeConn)
	if err != nil {
		fmt.Println("Failed to Channel ConsumerWithContext, err: ", err)
	}
	msgs, err := consumeClient.Consume(queue2, consumerTag2, false)
	if err != nil {
		fmt.Println("Failed to Consume msg, err: ", err)
	}
	log.Printf("[ConsumerWithContext] Waiting for messages.\n")
	for d := range msgs {
		log.Printf("Received a message: %s", string(d.Body))
		d.Ack(false)
	}
	err = consumeClient.Cancel(consumerTag2)
	if err != nil {
		fmt.Println("Failed to Cancel ConsumerWithContext, err: ", err)
	}
	err = consumeConn.Close()
	if err != nil {
		fmt.Println("Failed to Close ConsumerWithContext, err: ", err)
	}
}

// RabbitClient is used to keep track of the RabbitMQ connection
type RabbitClient struct {
	// The connection that is used
	conn *amqp.Connection
	// The channel that processes/sends Messages
	ch *amqp.Channel
}

func NewRabbitMQClient(conn *amqp.Connection) (RabbitClient, error) {
	ch, err := conn.Channel()
	if err != nil {
		return RabbitClient{}, err
	}
	if err := ch.Confirm(false); err != nil {
		return RabbitClient{}, err
	}

	return RabbitClient{
		conn: conn,
		ch:   ch,
	}, nil
}

func (rc RabbitClient) Close() error {
	return rc.ch.Close()
}

func (rc RabbitClient) Cancel(consumerTag string) error {
	return rc.ch.Cancel(consumerTag, false)
}

func (rc RabbitClient) CreateQueue(queueName string, durable, autoDelete bool) (amqp.Queue, error) {
	q, err := rc.ch.QueueDeclare(queueName, durable, autoDelete, false, false, nil)
	if err != nil {
		return amqp.Queue{}, nil
	}
	return q, nil
}

func (rc RabbitClient) CreateExchange(exchangeName, kind string) {
	err := rc.ch.ExchangeDeclare(exchangeName, kind, true, false, false, false, nil)
	if err != nil {
		fmt.Println("Failed to declare a exchange, err: ", err)
	}
}

func (rc RabbitClient) CreateBinding(name, binding, exchange string) error {
	return rc.ch.QueueBind(name, binding, exchange, false, nil)
}

func (rc RabbitClient) Send(ctx context.Context, exchange, routingKey string, options amqp.Publishing) error {
	_, err := rc.ch.PublishWithDeferredConfirmWithContext(ctx,
		exchange,
		routingKey,
		true,
		false,
		options,
	)
	if err != nil {
		return err
	}
	return nil
}

func (rc RabbitClient) Consume(queue, consumer string, autoAck bool) (<-chan amqp.Delivery, error) {
	return rc.ch.Consume(queue, consumer, autoAck, false, false, false, nil)
}

func (rc RabbitClient) ConsumeWithContext(ctx context.Context, queue, consumer string, autoAck bool) (<-chan amqp.Delivery, error) {
	return rc.ch.ConsumeWithContext(ctx, queue, consumer, autoAck, false, false, false, nil)
}
