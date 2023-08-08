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
	"net"

	"test/plugins/scenarios/grpc/api"

	"google.golang.org/grpc"

	_ "github.com/apache/skywalking-go"
)

type Echo struct {
	api.UnimplementedEchoServer
}

func (e *Echo) UnaryEcho(ctx context.Context, req *api.EchoRequest) (*api.EchoResponse, error) {
	log.Printf("Recved: %v", req.GetMessage())
	resp := &api.EchoResponse{Message: req.GetMessage()}
	return resp, nil
}

func main() {
	listen, err := net.Listen("tcp", ":9999")
	if err != nil {
		log.Fatal(err)
		return
	}

	s := grpc.NewServer()

	api.RegisterEchoServer(s, &Echo{})

	err = s.Serve(listen)
	if err != nil {
		log.Fatal(err)
		return
	}
}
