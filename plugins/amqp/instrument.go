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

package amqp

import (
	"embed"

	"github.com/apache/skywalking-go/plugins/core/instrument"
)

//go:embed *
var fs embed.FS

//skywalking:nocopy
type Instrument struct{}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "amqp"
}

func (i *Instrument) BasePackage() string {
	return "github.com/rabbitmq/amqp091-go"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "",
			PackageName: "amqp091",
			At: instrument.NewMethodEnhance("*Channel", "PublishWithDeferredConfirmWithContext",
				instrument.WithArgsCount(6),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "string"),
				instrument.WithArgType(2, "string"),
				instrument.WithArgType(5, "Publishing"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "*DeferredConfirmation"),
				instrument.WithResultType(1, "error"),
			),
			Interceptor: "ProducerInterceptor",
		},
		{
			PackagePath: "",
			PackageName: "amqp091",
			At: instrument.NewMethodEnhance("*Channel", "Consume",
				instrument.WithArgsCount(7),
				instrument.WithArgType(0, "string"),
				instrument.WithArgType(1, "string"),
				instrument.WithArgType(6, "Table"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "<-chan Delivery"),
				instrument.WithResultType(1, "error"),
			),
			Interceptor: "ConsumerInterceptor",
		},
		{
			PackagePath: "",
			PackageName: "amqp091",
			At: instrument.NewStaticMethodEnhance("DialConfig",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "string"),
				instrument.WithArgType(1, "Config"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "*Connection"),
				instrument.WithResultType(1, "error"),
			),
			Interceptor: "DialInterceptor",
		},
		{
			PackagePath: "",
			PackageName: "amqp091",
			At:          instrument.NewStructEnhance("Connection"),
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
