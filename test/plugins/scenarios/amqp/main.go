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

var (
	defaultConn    *Connection
	defaultChannel *amqp.Channel
)

const (
	URI          = "amqp://admin:123456@amqp-server:5672"
	exchangeName = "sw-exchange"
	queueName    = "sw-queue"
	keyName      = "sw-key"
	consumerName = "sw-consumer"
)

func Init() (err error) {
	defaultConn, err = Dial(URI)
	if err != nil {
		return fmt.Errorf("new mq conn err: %v", err)
	}

	defaultChannel, err = defaultConn.Channel()
	if err != nil {
		return fmt.Errorf("new mq channel err: %v", err)
	}
	return
}

func main() {
	if err := Init(); err != nil {
		log.Fatalf("amqp init err: %v", err)
	}
	if err := defaultChannel.ExchangeDeclare(exchangeName, "direct", true, false, false, false, nil); err != nil {
		log.Fatalf("create exchange err: %v", err)
	}
	if _, err := defaultChannel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatalf("create queue err: %v", err)
	}
	if err := defaultChannel.QueueBind(queueName, keyName, exchangeName, false, nil); err != nil {
		log.Fatalf("bind queue err: %v", err)
	}

	route := http.NewServeMux()
	route.HandleFunc("/execute", handlerFunc)

	route.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("ok"))
	})
	err := http.ListenAndServe(":8080", route)
	if err != nil {
		log.Fatalf("client start error: %v \n", err)
	}
}

func handlerFunc(writer http.ResponseWriter, request *http.Request) {
	_, err := defaultChannel.PublishWithDeferredConfirmWithContext(context.Background(), exchangeName, keyName, false, false,
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "text/plain",
			ContentEncoding: "",
			DeliveryMode:    amqp.Persistent,
			Priority:        0,
			AppId:           "skywalking-go",
			Body:            []byte("I love skywalking three thousand"),
		},
	)
	if err != nil {
		log.Fatalf("publish msg err: %v", err)
	}

	go func() {
		if err := NewConsumer(context.Background(), queueName, func(body []byte) error {
			fmt.Println("consume msg: " + string(body))
			return nil
		}); err != nil {
			log.Fatalf("consume err: %v", err)
		}
	}()

	_, _ = writer.Write([]byte("execute success"))
}

func NewConsumer(ctx context.Context, queue string, handler func([]byte) error) error {
	ch, err := defaultConn.Channel()
	if err != nil {
		return fmt.Errorf("new mq channel err: %v", err)
	}

	deliveries, err := ch.Consume(queue, consumerName, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume err: %v, queue: %s", err, queue)
	}

	for msg := range deliveries {
		select {
		case <-ctx.Done():
			_ = msg.Reject(true)
			return fmt.Errorf("context cancel")
		default:
		}
		err = handler(msg.Body)
		if err != nil {
			_ = msg.Reject(true)
			continue
		}
		_ = msg.Ack(false)
	}

	return nil
}

type Connection struct {
	*amqp.Connection
}

func Dial(url string) (*Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	connection := &Connection{
		Connection: conn,
	}

	go func() {
		for {
			reason, ok := <-connection.Connection.NotifyClose(make(chan *amqp.Error))
			// exit this goroutine if closed by developer
			if !ok {
				log.Println("connection closed")
				break
			}
			log.Printf("connection closed, reason: %v", reason)

			// reconnect if not closed by developer
			for {
				// wait 1s for reconnect
				time.Sleep(3 * time.Second)

				conn, err := amqp.Dial(url)
				if err == nil {
					connection.Connection = conn
					log.Println("reconnect success")
					break
				}

				log.Printf("reconnect failed, err: %v", err)
			}
		}
	}()

	return connection, nil
}
