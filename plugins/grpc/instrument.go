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
	"strconv"
	"strings"

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

// VersionChecker accepts grpc v1.x releases before v1.84.0. Since v1.84.0 the
// server merged processUnaryRPC/processStreamingRPC into processRPC and routes
// unary RPCs through serverStream, which is handled by the grpc-v184 plugin.
func (i *Instrument) VersionChecker(version string) bool {
	major, minor, ok := parseMajorMinor(version)
	if !ok {
		// The version is empty when grpc is replaced by a local module. Keep
		// instrumenting it with this plugin, as before the version split.
		return true
	}
	return major == 1 && minor < 84
}

// parseMajorMinor extracts the numeric major and minor parts of a module
// version such as "v1.83.2", "v1.84.0-dev" or a pseudo version.
func parseMajorMinor(version string) (major, minor int, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(version, "v"), ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minorPart := parts[1]
	if idx := strings.IndexFunc(minorPart, func(r rune) bool { return r < '0' || r > '9' }); idx >= 0 {
		minorPart = minorPart[:idx]
	}
	minor, err = strconv.Atoi(minorPart)
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
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
				instrument.WithArgType(0, "transport.ServerTransport")),
			Interceptor: "ServerHandleStreamInterceptor ",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Server", "sendResponse",
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerSendResponseInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Server", "processUnaryRPC",
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerUnaryInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Server", "processStreamingRPC",
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerStreamingInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*clientStream", "SendMsg",
				instrument.WithArgsCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "ClientSendMsgInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*clientStream", "RecvMsg",
				instrument.WithArgsCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "ClientRecvMsgInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*serverStream", "SendMsg",
				instrument.WithArgsCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "ServerSendMsgInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*serverStream", "RecvMsg",
				instrument.WithArgsCount(1),
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
