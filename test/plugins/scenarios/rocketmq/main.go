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
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"

	_ "github.com/apache/skywalking-go"
)

type testFunc func() error

const (
	uri   = "http://mqnamesrv:9876"
	retry = 2
	topic = "sw-topic"
	group = "sw-group"
	msg   = "I love skywalking %s thousand"
)

func main() {
	route := http.NewServeMux()
	route.HandleFunc("/execute", func(res http.ResponseWriter, req *http.Request) {
		TestProCon()
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

func TestProCon() {
	tests := []struct {
		name string
		fn   testFunc
	}{
		{"sendSyncMsg", sendSyncMsg},
		{"sendAsyncMsg", sendAsyncMsg},
		{"sendOneWayMsg", sendOneWayMsg},
	}
	for _, test := range tests {
		fmt.Printf("excute test case: %s\n", test.name)
		if subErr := test.fn(); subErr != nil {
			fmt.Printf("test case %s failed: %v", test.name, subErr)
		}
	}
}

func sendSyncMsg() error {
	p, err := rocketmq.NewProducer(
		producer.WithNsResolver(primitive.NewPassthroughResolver([]string{uri})),
		producer.WithRetry(retry),
		producer.WithGroupName(group),
	)
	if err != nil {
		fmt.Printf("new producer error: %s\n", err.Error())
		return err
	}
	err = p.Start()
	if err != nil {
		fmt.Printf("start producer error: %s\n", err.Error())
		return err
	}
	var msgs []*primitive.Message
	for i := 1; i < 2; i++ {
		msgs = append(msgs, primitive.NewMessage(
			topic,
			[]byte(fmt.Sprintf(msg, strconv.Itoa(i)))),
		)
	}

	res, err := p.SendSync(context.Background(), msgs...)
	if err != nil {
		fmt.Printf("send message error: %s\n", err)
		return err
	} else {
		fmt.Printf("send message success: result=%s\n", res.String())
	}
	err = p.Shutdown()
	if err != nil {
		fmt.Printf("shutdown producer error: %s\n", err.Error())
		return err
	}
	consumerMsg()
	return nil
}

func sendAsyncMsg() error {
	p, err := rocketmq.NewProducer(
		producer.WithNsResolver(primitive.NewPassthroughResolver([]string{uri})),
		producer.WithRetry(retry),
		producer.WithGroupName(group),
	)

	if err != nil {
		fmt.Printf("new producer error: %s\n", err.Error())
		return err
	}
	err = p.Start()
	if err != nil {
		fmt.Printf("start producer error: %s\n", err.Error())
		return err
	}
	var wg sync.WaitGroup
	for i := 1; i < 2; i++ {
		wg.Add(1)
		err = p.SendAsync(context.Background(),
			func(ctx context.Context, result *primitive.SendResult, e error) {
				if e != nil {
					fmt.Printf("receive message error: %s\n", err)
				} else {
					fmt.Printf("send message success: result=%s\n", result.String())
				}
				wg.Done()
			}, primitive.NewMessage(topic, []byte(fmt.Sprintf(msg, strconv.Itoa(i)))))

		if err != nil {
			fmt.Printf("send message error: %s\n", err)
			return err
		}
	}
	wg.Wait()
	err = p.Shutdown()
	if err != nil {
		fmt.Printf("shutdown producer error: %s\n", err.Error())
		return err
	}
	consumerMsg()
	return nil
}

func sendOneWayMsg() error {
	p, err := rocketmq.NewProducer(
		producer.WithNsResolver(primitive.NewPassthroughResolver([]string{uri})),
		producer.WithRetry(retry),
		producer.WithGroupName(group),
	)
	if err != nil {
		fmt.Printf("new producer error: %s\n", err.Error())
		return err
	}
	err = p.Start()
	if err != nil {
		fmt.Printf("start producer error: %s\n", err.Error())
		return err
	}
	var msgs []*primitive.Message
	for i := 1; i < 2; i++ {
		msgs = append(msgs, primitive.NewMessage(
			topic,
			[]byte(fmt.Sprintf(msg, strconv.Itoa(i)))),
		)
	}
	err = p.SendOneWay(context.Background(), msgs...)
	if err != nil {
		fmt.Printf("send_one_way message error: %s\n", err)
		return err
	}
	err = p.Shutdown()
	if err != nil {
		fmt.Printf("shutdown producer error: %s\n", err.Error())
		return err
	}
	consumerMsg()
	return nil
}

func consumerMsg() {
	var err error
	c, err := rocketmq.NewPushConsumer(
		consumer.WithGroupName(group),
		consumer.WithNsResolver(primitive.NewPassthroughResolver([]string{uri})),
	)
	if err != nil {
		fmt.Printf("new consumer error: %s\n", err.Error())
	}
	err = c.Subscribe(topic, consumer.MessageSelector{}, func(ctx context.Context,
		msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for i := range msgs {
			fmt.Printf("subscribe callback: %v \n", msgs[i])
		}
		return consumer.ConsumeSuccess, nil
	})
	if err != nil {
		fmt.Println(err.Error())
	}
	err = c.Start()
	if err != nil {
		fmt.Println(err.Error())
	}
	time.Sleep(time.Second)
}
