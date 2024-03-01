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

	"github.com/apache/pulsar-client-go/pulsar"

	_ "github.com/apache/skywalking-go"
)

type testFunc func() error

var (
	url           = "pulsar://pulsar-server:6650"
	msg           = "I love skywalking 3 thousand"
	topic1        = "sw-topic-1"
	topic2        = "sw-topic-2"
	subscription1 = "sw-subscription-1"
	subscription2 = "sw-subscription-2"
	client        pulsar.Client
)

func main() {
	var err error
	client, err = pulsar.NewClient(pulsar.ClientOptions{
		URL: url,
	})
	if err != nil {
		panic(err)
	}

	route := http.NewServeMux()
	route.HandleFunc("/execute", func(res http.ResponseWriter, req *http.Request) {
		testProCon()
		_, _ = res.Write([]byte("execute success"))
	})
	route.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	})
	fmt.Println("start client")

	err = http.ListenAndServe(":8080", route)
	if err != nil {
		fmt.Printf("client start error: %v \n", err)
	}
}

func consumerHelper() {
	go consumerMsg(context.Background(), topic1, subscription1)
	go consumerMsg(context.Background(), topic2, subscription2)
}

func testProCon() {
	tests := []struct {
		name string
		fn   testFunc
	}{
		{"sendMsg", sendMsg},
		{"sendAsyncMsg", sendAsyncMsg},
	}
	for _, test := range tests {
		fmt.Printf("excute test case: %s\n", test.name)
		if subErr := test.fn(); subErr != nil {
			fmt.Printf("test case %s failed: %v\n", test.name, subErr)
		}
	}
}

func sendMsg() error {
	go consumerHelper()
	ctx := context.Background()

	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic1,
	})
	if err != nil {
		return err
	}
	if msgId, err := producer.Send(ctx, &pulsar.ProducerMessage{
		Payload: []byte(msg),
	}); err != nil {
		return err
	} else {
		log.Println("Published message: ", msgId)
	}
	time.Sleep(time.Second)
	return nil
}

func sendAsyncMsg() error {
	time.Sleep(time.Second)
	ctx := context.Background()
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic2,
	})
	if err != nil {
		return err
	}

	producer.SendAsync(ctx, &pulsar.ProducerMessage{
		Payload: []byte(msg),
	}, func(id pulsar.MessageID, message *pulsar.ProducerMessage, err error) {
		log.Printf("ID = %v, Properties = %v", id, message.Properties)
	})
	return nil
}

func consumerMsg(ctx context.Context, topic string, subscription string) {
	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: subscription,
	})
	if err != nil {
		log.Fatal(err)
	}

	msg, err := consumer.Receive(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Received message msgId: %#v -- content: '%s'\n",
		msg.ID(), string(msg.Payload()))
	if err := consumer.Unsubscribe(); err != nil {
		log.Fatal(err)
	}
}
