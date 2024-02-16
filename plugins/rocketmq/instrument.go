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

package rocketmq

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
	return "rocketmq"
}

func (i *Instrument) BasePackage() string {
	return "github.com/apache/rocketmq-client-go/v2"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "producer",
			At: instrument.NewMethodEnhance("*defaultProducer", "SendSync",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "...*primitive.Message"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "*primitive.SendResult"),
				instrument.WithResultType(1, "error")),
			Interceptor: "SendSyncInterceptor",
		},
		{
			PackagePath: "producer",
			At: instrument.NewMethodEnhance("*defaultProducer", "SendAsync",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "func(context.Context, *primitive.SendResult, error)"),
				instrument.WithArgType(2, "...*primitive.Message"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "SendASyncInterceptor",
		},
		{
			PackagePath: "producer",
			At: instrument.NewMethodEnhance("*defaultProducer", "SendOneWay",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "msgs ...*primitive.Message"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "error")),
			Interceptor: "SendOneWayInterceptor",
		},
		{
			PackagePath: "consumer",
			At: instrument.NewMethodEnhance("*pushConsumer", "consumeInner",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "[]*primitive.MessageExt"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "ConsumeResult"),
				instrument.WithResultType(1, "error")),
			Interceptor: "SwConsumerInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
