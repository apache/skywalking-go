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
	"time"

	hello "github.com/go-micro/examples/greeter/srv/proto/hello"

	"go-micro.dev/v4"
	"go-micro.dev/v4/client"

	_ "github.com/apache/skywalking-go"
)

func main() {
	// create a new service
	service := micro.NewService()
	service.Init()
	cl := hello.NewSayService("go.micro.srv.greeter", service.Client())

	// go-micro serves a call without a service name when the address is given,
	// so the agent must not fail it, even though it cannot trace it. It uses its
	// own client, so that the traced call below does not reuse its connection.
	addressService := micro.NewService(micro.Client(client.NewClient()))
	addressReq := addressService.Client().NewRequest("", "Say.Hello", &hello.Request{Name: "address"})
	var err error
	// retry while the server is starting
	for i := 0; i < 10; i++ {
		if err = addressService.Client().Call(context.Background(), addressReq, &hello.Response{},
			client.WithAddress("localhost:8888")); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		log.Fatalf("call by address error: %v", err)
	}

	http.HandleFunc("/consumer", func(writer http.ResponseWriter, request *http.Request) {
		resp, err := cl.Hello(context.Background(), &hello.Request{Name: "John"})
		if err != nil {
			_, _ = writer.Write([]byte(err.Error()))
			return
		}
		_, _ = writer.Write([]byte(resp.Msg))
	})
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("success"))
	})
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
