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

package sarama

import (
	"embed"

	"github.com/hashicorp/go-version"

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
	return "sarama"
}

func (i *Instrument) BasePackage() string {
	return "github.com/Shopify/sarama"
}

func (i *Instrument) VersionChecker(pluginVersion string) bool {
	// https://github.com/Shopify/sarama/releases/tag/v1.27.0
	// KIP-42 producer and consumer interceptors were introduced since v1.27.0
	v1, _ := version.NewVersion(pluginVersion)
	v2, _ := version.NewVersion("v1.27.0")
	return v1.GreaterThanOrEqual(v2)
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			At: instrument.NewStaticMethodEnhance(
				"newAsyncProducer",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "Client"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "AsyncProducer"),
				instrument.WithResultType(1, "error"),
			),
			Interceptor: "AsyncProducerInterceptor",
		},
		{
			At: instrument.NewStaticMethodEnhance(
				"newConsumer",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "Client"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "Consumer"),
				instrument.WithResultType(1, "error"),
			),
			Interceptor: "ConsumerInterceptor",
		},
		{
			At: instrument.NewMethodEnhance(
				"*syncProducer",
				"SendMessage",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "*ProducerMessage"),
				instrument.WithResultCount(3),
				instrument.WithResultType(0, "int32"),
				instrument.WithResultType(1, "int64"),
				instrument.WithResultType(2, "error"),
			),
			Interceptor: "SendMessageInterceptor",
		},
		//{
		//	At: instrument.NewMethodEnhance(
		//		"*syncProducer",
		//		"SendMessages",
		//		instrument.WithArgsCount(1),
		//		instrument.WithArgType(0, "[]*ProducerMessage"),
		//		instrument.WithResultCount(1),
		//		instrument.WithResultType(0, "error"),
		//	),
		//	Interceptor: "SendMessagesInterceptor",
		//},
		{
			At: instrument.NewStructEnhance("syncProducer"),
		},
		{
			At: instrument.NewStaticMethodEnhance(
				"NewSyncProducer",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "[]string"),
				instrument.WithArgType(1, "*Config"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "SyncProducer"),
				instrument.WithResultType(1, "error"),
			),
			Interceptor: "NewSyncProducerInterceptor",
		},
		{
			At: instrument.NewStaticMethodEnhance(
				"NewSyncProducerFromClient",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "Client"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "SyncProducer"),
				instrument.WithResultType(1, "error"),
			),
			Interceptor: "NewSyncProducerFromClientInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
