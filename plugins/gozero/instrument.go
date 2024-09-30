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

package gozero

import (
	"embed"
	"github.com/apache/skywalking-go/plugins/core/instrument"
	"strings"
)

//go:embed *
var fs embed.FS

type Instrument struct {
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "gozero"
}

func (i *Instrument) BasePackage() string {
	return "github.com/zeromicro/go-zero"
}

func (i *Instrument) VersionChecker(version string) bool {
	return strings.HasPrefix(version, "v1.")
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "rest",
			At:          instrument.NewMethodEnhance("*Server", "Start"),
			Interceptor: "ServerMiddlewareInterceptor",
		},
		{
			PackagePath: "zrpc",
			At:          instrument.NewMethodEnhance("*RpcServer", "Start"),
			Interceptor: "ServerMiddlewareInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Debug",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "...any"),
			),
			Interceptor: "LoggerDebugInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Debugf",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "string"),
				instrument.WithArgType(1, "...any"),
			),
			Interceptor: "LoggerDebugInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Error",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "...any"),
			),
			Interceptor: "LoggerErrorInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Errorf",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "string"),
				instrument.WithArgType(1, "...any"),
			),
			Interceptor: "LoggerErrorInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Info",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "...any"),
			),
			Interceptor: "LoggerInfoInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Infof",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "string"),
				instrument.WithArgType(1, "...any"),
			),
			Interceptor: "LoggerInfoInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Slow",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "...any")),
			Interceptor: "LoggerSlowInterceptor",
		},
		{
			PackagePath: "core/logx",
			At: instrument.NewStaticMethodEnhance("Slowf",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "string"),
				instrument.WithArgType(1, "...any"),
			),
			Interceptor: "LoggerSlowInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
