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

package kratosv2

import (
	"embed"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/instrument"
)

//go:embed *
var fs embed.FS

type Instrument struct {
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "kratosv2"
}

func (i *Instrument) BasePackage() string {
	return "github.com/go-kratos/kratos/v2"
}

func (i *Instrument) VersionChecker(version string) bool {
	return strings.HasPrefix(version, "v2")
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		// http transport
		{
			PackagePath: "transport/http",
			At:          instrument.NewStructEnhance("Server"),
		},
		{
			PackagePath: "transport/http",
			At:          instrument.NewStructEnhance("clientOptions"),
		},
		{
			PackagePath: "transport/http",
			At: instrument.NewStaticMethodEnhance("NewServer",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "...ServerOption"),
				instrument.WithResultCount(1), instrument.WithResultType(0, "*Server")),
			Interceptor: "NewServerInterceptor",
		},
		{
			PackagePath: "transport/http",
			At: instrument.NewStaticMethodEnhance("Middleware",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "...middleware.Middleware"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "ServerOption")),
			Interceptor: "ServerMiddlewareInterceptor",
		},
		{
			PackagePath: "transport/http",
			At: instrument.NewStaticMethodEnhance("NewClient",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "...ClientOption"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "*Client"), instrument.WithResultType(1, "error")),
			Interceptor: "NewClientInterceptor",
		},
		{
			PackagePath: "transport/http",
			At: instrument.NewStaticMethodEnhance("WithMiddleware",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "...middleware.Middleware"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "ClientOption")),
			Interceptor: "ClientMiddlewareInterceptor",
		},
		// grpc transport
		{
			PackagePath: "transport/grpc",
			At:          instrument.NewStructEnhance("Server"),
		},
		{
			PackagePath: "transport/grpc",
			At:          instrument.NewStructEnhance("clientOptions"),
		},
		{
			PackagePath: "transport/grpc",
			At: instrument.NewStaticMethodEnhance("NewServer",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "...ServerOption"),
				instrument.WithResultCount(1), instrument.WithResultType(0, "*Server")),
			Interceptor: "NewServerInterceptor",
		},
		{
			PackagePath: "transport/grpc",
			At: instrument.NewStaticMethodEnhance("Middleware",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "...middleware.Middleware"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "ServerOption")),
			Interceptor: "MiddlewareInterceptor",
		},
		{
			PackagePath: "transport/grpc",
			At: instrument.NewStaticMethodEnhance("unaryClientInterceptor",
				instrument.WithArgType(0, "[]middleware.Middleware")),
			Interceptor: "UnaryClientInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
