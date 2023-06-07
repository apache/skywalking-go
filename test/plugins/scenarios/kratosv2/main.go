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
	nativeHTTP "net/http"

	nativeGRPC "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/go-kratos/examples/helloworld/helloworld"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"

	_ "github.com/apache/skywalking-go"
)

var httpClient *http.Client
var gRPCClient helloworld.GreeterClient

type server struct {
	helloworld.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	return &helloworld.HelloReply{Message: fmt.Sprintf("Hello %s", in.Name)}, nil
}

func consumerHandler(w nativeHTTP.ResponseWriter, r *nativeHTTP.Request) {
	var resp *helloworld.HelloReply
	err := httpClient.Invoke(context.Background(), "GET", "/helloworld/test", nil, &resp)
	if err != nil {
		log.Printf("request provider error: %v", err)
		w.WriteHeader(nativeHTTP.StatusInternalServerError)
		return
	}
	hello, err := gRPCClient.SayHello(context.Background(), &helloworld.HelloRequest{Name: "kratos"})
	if err != nil {
		log.Printf("request provider error: %v", err)
		w.WriteHeader(nativeHTTP.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(resp.Message + hello.Message))
}

func main() {
	httpSvr := http.NewServer(http.Address(":8000"))
	grpcSvr := grpc.NewServer(grpc.Address(":9000"))
	if c, err := http.NewClient(context.Background(), http.WithEndpoint("http://localhost:8000")); err != nil {
		log.Fatalf("creating HTTP client error: %v", err)
	} else {
		httpClient = c
	}
	if dial, err := grpc.Dial(context.Background(),
		grpc.WithEndpoint("localhost:9000"),
		grpc.WithOptions(nativeGRPC.WithTransportCredentials(insecure.NewCredentials()))); err != nil {
		log.Fatalf("creating gRPC client error: %v", err)
	} else {
		gRPCClient = helloworld.NewGreeterClient(dial)
	}

	s := &server{}
	helloworld.RegisterGreeterHTTPServer(httpSvr, s)
	helloworld.RegisterGreeterServer(grpcSvr, s)

	app := kratos.New(
		kratos.Name("krago-test"),
		kratos.Server(httpSvr, grpcSvr),
	)

	nativeHTTP.HandleFunc("/consumer", consumerHandler)
	nativeHTTP.HandleFunc("/health", func(writer nativeHTTP.ResponseWriter, request *nativeHTTP.Request) {
		writer.WriteHeader(nativeHTTP.StatusOK)
	})

	go func() {
		_ = nativeHTTP.ListenAndServe(":8080", nil)
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
