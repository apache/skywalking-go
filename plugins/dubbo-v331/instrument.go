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

package dubbov331

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
	return "dubbo"
}

func (i *Instrument) BasePackage() string {
	return "dubbo.apache.org/dubbo-go/v3"
}

// VersionChecker accepts dubbo-go v3.3.1 and later, where the
// filter/graceful_shutdown package imports protocol/base and protocol/result.
// Older releases are handled by the dubbo plugin.
func (i *Instrument) VersionChecker(version string) bool {
	v, ok := parseVersion(version)
	return ok && atLeastV331(v)
}

// parseVersion extracts the numeric major, minor, and patch parts of a module
// version, such as v3.3.1 or a pseudo version based on it.
func parseVersion(version string) (v [3]int, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(version, "v"), ".", 3)
	if len(parts) < 3 {
		return v, false
	}
	for i, part := range parts {
		if idx := strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }); idx >= 0 {
			part = part[:idx]
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func atLeastV331(v [3]int) bool {
	if v[0] != 3 {
		return v[0] > 3
	}
	if v[1] != 3 {
		return v[1] > 3
	}
	return v[2] >= 1
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "filter/graceful_shutdown",
			At: instrument.NewMethodEnhance("*consumerGracefulShutdownFilter", "Invoke",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "base.Invoker"),
				instrument.WithArgType(2, "base.Invocation"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "result.Result"),
			),
			Interceptor: "ClientInterceptor",
		},
		{
			PackagePath: "filter/graceful_shutdown",
			At: instrument.NewMethodEnhance("*providerGracefulShutdownFilter", "Invoke",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "context.Context"),
				instrument.WithArgType(1, "base.Invoker"),
				instrument.WithArgType(2, "base.Invocation"),
				instrument.WithResultCount(1),
				instrument.WithResultType(0, "result.Result"),
			),
			Interceptor: "ServerInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
