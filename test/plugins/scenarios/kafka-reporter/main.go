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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"net"
	"net/http"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	"strconv"
	"time"

	_ "github.com/apache/skywalking-go"
	"github.com/segmentio/kafka-go"
)

var (
	url              = "kafka-server:9092"
	topic_segment    = "skywalking-segments"
	topic_meter      = "skywalking-meters"
	topic_logging    = "skywalking-logging"
	topic_management = "skywalking-managements"
	ctx              = context.Background()
	reader           *kafka.Reader

	mockOap     = "oap:19876"
	traceClient agentv3.TraceSegmentReportServiceClient
)

func executeHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("success"))
}

func main() {
	createTopic()
	reader = newKafkaReader()
	defer reader.Close()
	consumerAndReportMockCollector()

	http.HandleFunc("/execute", executeHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}

func consumerAndReportMockCollector() {
	go func() {
		grpcConn, err := grpc.Dial(mockOap, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if err != nil {
			fmt.Println("grpc dial err:", err)
		}
		traceClient = agentv3.NewTraceSegmentReportServiceClient(grpcConn)
		for {
			message, err := reader.ReadMessage(ctx)
			if err != nil {
				fmt.Printf("Error reading message: %s\n", err)
				time.Sleep(1 * time.Second)
				continue
			}
			fmt.Printf("consumer|topic=%s, partition=%d, offset=%d, key=%s, header=%s\n",
				message.Topic, message.Partition, message.Offset, string(message.Key), message.Headers)
			segment := &agentv3.SegmentObject{}
			if err := proto.Unmarshal(message.Value, segment); err != nil {
				fmt.Printf("Error unmarshalling message: %s\n", err)
				continue
			}
			fmt.Printf("Forwarding SegmentObject: TraceID=%s, SegmentID=%s, Spans=%d\n", segment.GetTraceId(), segment.GetTraceSegmentId(), len(segment.GetSpans()))
			stream, err := traceClient.Collect(ctx)
			if err != nil {
				fmt.Printf("Error creating stream: %s\n", err)
				continue
			}
			if err := stream.Send(segment); err != nil {
				fmt.Printf("Error sending segment to mock-collector: %v\n", err)
				_, closeErr := stream.CloseAndRecv()
				if closeErr != nil {
					fmt.Printf("Error closing segment stream after send error: %v\n", closeErr)
				}
				continue
			}
			if _, err := stream.CloseAndRecv(); err != nil {
				if err.Error() != "EOF" {
					fmt.Printf("Error closing and receiving from segment stream: %v\n", err)
				} else {
					fmt.Printf("Segment stream closed by server (EOF).\n")
				}
			} else {
				fmt.Printf("SegmentObject sent successfully to mock-collector.\n")
			}
		}
	}()
}

func newKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{url},
		Topic:          topic_segment,
		CommitInterval: 1 * time.Second,
	})
}

func createTopic() {
	conn, err := kafka.Dial("tcp", url)
	if err != nil {
		fmt.Printf("createTopic, dial err: %w", err)
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		fmt.Printf("createTopic, conn.Controller: %w", err)
	}
	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		fmt.Printf("kafka.Dial error: %w", err)
	}
	defer controllerConn.Close()
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic_segment,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
		{
			Topic:             topic_meter,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
		{
			Topic:             topic_logging,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
		{
			Topic:             topic_management,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}
	_ = controllerConn.SetDeadline(time.Now().Add(15 * time.Second))
	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		fmt.Printf("createTopic error: %w", err)
	} else {
		fmt.Printf("createTopic success\n")
		time.Sleep(5 * time.Second)
	}
}
