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

	hello "github.com/go-micro/examples/greeter/srv/proto/hello"

	"go-micro.dev/v4"

	_ "github.com/apache/skywalking-go"
)

func main() {
	// create a new service
	service := micro.NewService()
	service.Init()
	cl := hello.NewSayService("go.micro.srv.greeter", service.Client())

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
