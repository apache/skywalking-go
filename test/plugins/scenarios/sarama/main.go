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
	"log"
	"net/http"

	"github.com/Shopify/sarama"

	_ "github.com/apache/skywalking-go"
)

var (
	producer sarama.AsyncProducer
	consumer sarama.Consumer
)

type testFunc func(ctx context.Context) error

func executeHandler(w http.ResponseWriter, r *http.Request) {
	testCases := []struct {
		name string
		fn   testFunc
	}{
		{"produce", testProduce},
		{"consume", testConsume},
	}

	for _, test := range testCases {
		log.Printf("excute test case %s", test.name)
		if err := test.fn(r.Context()); err != nil {
			log.Fatalf("test case %s failed: %v", test.name, err)
		}
	}
	_, _ = w.Write([]byte("execute kafka op success"))
}

func testProduce(ctx context.Context) error {
	producer.Input() <- &sarama.ProducerMessage{
		Topic: "sarama_auto_instrument",
		Key:   nil,
		Value: sarama.StringEncoder("this is a test msg"),
	}
	defer producer.AsyncClose()
	return nil
}

func testConsume(ctx context.Context) error {
	c, _ := consumer.ConsumePartition("sarama_auto_instrument", 3, 0)
	for i := int64(0); i < 10; i++ {
		select {
		case _ = <-c.Messages():
			continue
		case _ = <-c.Errors():
			break
		}
	}

	return nil
}

func main() {
	conf := sarama.NewConfig()
	conf.Version = sarama.V2_8_1_0
	producer, _ = sarama.NewAsyncProducer([]string{"kafka-server:9092"}, conf)
	consumer, _ = sarama.NewConsumer([]string{"kafka-server:9092"}, conf)

	http.HandleFunc("/execute", executeHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}
