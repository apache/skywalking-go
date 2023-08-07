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

	"test/plugins/scenarios/grpc/api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "github.com/apache/skywalking-go"
)

func unary(client api.EchoClient, writer http.ResponseWriter) {
	resp, err := client.UnaryEcho(context.Background(), &api.EchoRequest{Message: "Unary Echo"})
	if err != nil {
		writer.WriteHeader(500)
		_, _ = writer.Write([]byte(err.Error()))
		log.Println(err)
	}

	log.Println(resp.GetMessage())

	_, _ = writer.Write([]byte(resp.String()))
}

func main() {
	conn, err := grpc.Dial("127.0.0.1:9999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := api.NewEchoClient(conn)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	})

	http.HandleFunc("/consumer", func(writer http.ResponseWriter, request *http.Request) {
		unary(client, writer)
	})

	_ = http.ListenAndServe(":8080", nil)
}
