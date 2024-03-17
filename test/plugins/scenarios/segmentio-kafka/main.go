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
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"

	_ "github.com/apache/skywalking-go"
)

type testFunc func() error

var (
	url    = "kafka-server:9092"
	topic  = "sw-topic"
	msg    = "I love skywalking 3 thousand"
	ctx    = context.Background()
	writer *kafka.Writer
	reader *kafka.Reader
)

func main() {
	writer = newKafkaWriter(topic)
	defer writer.Close()
	reader = newKafkaReader()
	defer reader.Close()
	consumerHelper()

	route := http.NewServeMux()
	route.HandleFunc("/execute", func(res http.ResponseWriter, req *http.Request) {
		testProduceConsume()
		_, _ = res.Write([]byte("execute success"))
	})
	route.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	})
	fmt.Println("start client")
	err := http.ListenAndServe(":8080", route)
	if err != nil {
		fmt.Printf("client start error: %v \n", err)
	}
}

func testProduceConsume() {
	tests := []struct {
		name string
		fn   testFunc
	}{
		{"simpleMsg", simpleMsg},
	}
	for _, test := range tests {
		fmt.Printf("excute test case: %s\n", test.name)
		if subErr := test.fn(); subErr != nil {
			fmt.Printf("test case %s failed: %v", test.name, subErr)
		}
	}
}

func simpleMsg() error {
	if err := writer.WriteMessages(ctx, kafka.Message{
		Value: []byte(msg),
	}); err != nil {
		log.Println("simpleMsg WriteMessages error")
		return err
	}
	return nil
}

func consumerHelper() {
	go func() {
		for {
			if message, err := reader.ReadMessage(ctx); err != nil {
				log.Fatal("consumer msg error: ", err)
			} else {
				fmt.Printf("consumer|topic=%s, partition=%d, offset=%d, key=%s, value=%s, header=%s\n",
					message.Topic, message.Partition, message.Offset, string(message.Key), string(message.Value), message.Headers)
			}
		}
	}()
}

func newKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{url},
		Topic:          topic,
		CommitInterval: 1 * time.Second,
	})
}

func newKafkaWriter(topic string) *kafka.Writer {
	createTopic()
	return &kafka.Writer{
		Addr:  kafka.TCP(url),
		Topic: topic,
	}
}

func createTopic() {
	conn, err := kafka.Dial("tcp", url)
	if err != nil {
		log.Fatal(fmt.Errorf("createTopic, Dial: %w", err))
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		err = fmt.Errorf("createTopic, conn.Controller: %w", err)
		log.Fatal(err)
	}
	conn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		log.Fatal("kafka.Dial error: ", err)
	}
	conn.SetDeadline(time.Now().Add(time.Second))
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}
	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		log.Fatal(fmt.Errorf("createTopic error: %w", err))
	}
}
