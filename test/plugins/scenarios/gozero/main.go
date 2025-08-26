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
	"net/http"

	"test/plugins/scenarios/gozero/protos/proto"

	_ "github.com/apache/skywalking-go"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Config struct {
	zrpc.RpcServerConf
}

type ServiceContext struct {
	Config Config
}

func NewServiceContext(c Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}

func main() {
	server, _ := rest.NewServer(rest.RestConf{Port: 8999})

	server.AddRoutes([]rest.Route{
		{
			Handler: func(writer http.ResponseWriter, request *http.Request) {
				ctx := request.Context()
				clientConf := zrpc.RpcClientConf{}
				conf.FillDefault(&clientConf)
				clientConf.Endpoints = []string{"127.0.0.1:8089"}
				conn := zrpc.MustNewClient(clientConf)

				// 创建 gRPC 客户端
				client := proto.NewGreeterClient(conn.Conn())

				r, err := client.SayHello(ctx, &proto.Hello{Name: "World"})
				// 调用 gRPC 方法
				if err != nil {
					logc.Errorf(ctx, "could not greet: %v", err)
				}
				writer.Write([]byte(r.Message))
			},
			Method: http.MethodGet,
			Path:   "/ping",
		},
		{
			Handler: func(writer http.ResponseWriter, r *http.Request) {
				writer.WriteHeader(http.StatusOK)
			},
			Method: http.MethodGet,
			Path:   "/health",
		},
	})
	go server.Start()

	svcCtx := NewServiceContext(Config{})
	s := NewGreeterServer(svcCtx)
	rpcs, _ := zrpc.NewServer(zrpc.RpcServerConf{ListenOn: ":8089"}, func(gs *grpc.Server) {
		proto.RegisterGreeterServer(gs, s)
		reflection.Register(gs)
	})

	rpcs.Start()

}

type GreeterServer struct {
	svcCtx *ServiceContext
	proto.UnimplementedGreeterServer
}

func NewGreeterServer(svcCtx *ServiceContext) *GreeterServer {
	return &GreeterServer{
		svcCtx: svcCtx,
	}
}

func (s *GreeterServer) SayHello(ctx context.Context, in *proto.Hello) (*proto.Reply, error) {
	return &proto.Reply{Message: "world"}, nil
}
