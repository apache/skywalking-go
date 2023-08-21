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

package grpc

import (
	"embed"

	"github.com/apache/skywalking-go/plugins/core/instrument"
)

//go:embed *
var fs embed.FS

//skywalking:nocopy
type Instrument struct {
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "grpc"
}

func (i *Instrument) BasePackage() string {
	return "google.golang.org/grpc"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "",
			At:          instrument.NewStructEnhance("clientStream"),
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*ClientConn", "Invoke",
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "string")),
			Interceptor: "ClientUnaryInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*ClientConn", "NewStream",
				instrument.WithArgType(0, "context.Context"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "ClientStream")),
			Interceptor: "ClientStreamingInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Server", "handleStream",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "transport.ServerTransport")),
			Interceptor: "ServerHandleStreamInterceptor ",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Server", "sendResponse",
				instrument.WithArgsCount(6),
				instrument.WithArgType(0, "transport.ServerTransport"),
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerSendResponseInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Server", "processUnaryRPC",
				instrument.WithArgsCount(5),
				instrument.WithArgType(0, "transport.ServerTransport"),
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerUnaryInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Server", "processStreamingRPC",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "transport.ServerTransport"),
				instrument.WithArgType(1, "*transport.Stream")),
			Interceptor: "ServerStreamingInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*clientStream", "SendMsg",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "interface{}"),
				instrument.WithResultType(0, "error")),
			Interceptor: "ClientSendMsgInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*clientStream", "RecvMsg",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "interface{}"),
				instrument.WithResultType(0, "error")),
			Interceptor: "ClientRecvMsgInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*serverStream", "SendMsg",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "interface{}"),
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerSendMsgInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*serverStream", "RecvMsg",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "interface{}"),
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerRecvMsgInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*clientStream", "CloseSend",
				instrument.WithResultType(0, "error")),
			Interceptor: "ClientCloseSendInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*clientStream", "finish",
				instrument.WithArgType(0, "error")),
			Interceptor: "ClientFinishInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
