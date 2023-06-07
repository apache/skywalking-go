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

package microv4

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
	return "microv4"
}

func (i *Instrument) BasePackage() string {
	return "go-micro.dev/v4"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "",
			PackageName: "micro",
			At: instrument.NewStaticMethodEnhance("NewService",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "...Option"),
				instrument.WithResultCount(1), instrument.WithResultType(0, "Service")),
			Interceptor: "NewServiceInterceptor",
		},
		{
			PackagePath: "client",
			At: instrument.NewMethodEnhance("*rpcClient", "next",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "Request"), instrument.WithArgType(1, "CallOptions"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "selector.Next"), instrument.WithResultType(1, "error")),
			Interceptor: "NextInterceptor",
		},
		{
			PackagePath: "server",
			At: instrument.NewMethodEnhance("*router", "ServeRequest",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "Request"), instrument.WithArgType(2, "Response"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "ServeRequestInterceptor",
		},
		{
			PackagePath: "util/socket",
			At:          instrument.NewStructEnhance("Socket"),
		},
		{
			PackagePath: "util/socket",
			At: instrument.NewMethodEnhance("*Socket", "Accept",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "*transport.Message"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "AcceptInterceptor",
		},
		{
			PackagePath: "util/socket",
			At: instrument.NewMethodEnhance("*Socket", "Close",
				instrument.WithArgsCount(0),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "CloseInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
