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

package goredisv9

import (
	"embed"
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
	return "go-redisv9"
}

func (i *Instrument) BasePackage() string {
	return "github.com/redis/go-redis/v9"
}

func (i *Instrument) VersionChecker(version string) bool {
	return strings.HasPrefix(version, "v9.")
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackageName: "redis",
			PackagePath: "",
			At: instrument.NewStaticMethodEnhance("NewUniversalClient",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "*UniversalOptions"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "UniversalClient")),
			Interceptor: "GoRedisInterceptor",
		},
		{
			PackageName: "redis",
			PackagePath: "",
			At: instrument.NewStaticMethodEnhance("NewClient",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "*Options"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "*Client")),
			Interceptor: "GoRedisInterceptor",
		},
		{
			PackageName: "redis",
			PackagePath: "",
			At: instrument.NewStaticMethodEnhance("NewFailoverClient",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "*FailoverOptions"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "*Client")),
			Interceptor: "GoRedisInterceptor",
		},
		{
			PackageName: "redis",
			PackagePath: "",
			At: instrument.NewStaticMethodEnhance("NewClusterClient",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "*ClusterOptions"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "*ClusterClient")),
			Interceptor: "GoRedisInterceptor",
		},
		{
			PackageName: "redis",
			PackagePath: "",
			At: instrument.NewStaticMethodEnhance("NewRing",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "*RingOptions"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "*Ring")),
			Interceptor: "GoRedisInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
