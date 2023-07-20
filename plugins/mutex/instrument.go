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

package mutex

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

func (r *Instrument) Name() string {
	return "mutex"
}

func (r *Instrument) BasePackage() string {
	return "sync"
}

func (r *Instrument) VersionChecker(version string) bool {
	return true
}

func (r *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Mutex", "Lock",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "LockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Mutex", "TryLock",
				instrument.WithArgsCount(0),
				instrument.WithResultCount(1), instrument.WithResultType(0, "bool")),
			Interceptor: "TryLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Mutex", "Unlock",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "UnLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*RWMutex", "RLock",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "RWRLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*RWMutex", "TryRLock",
				instrument.WithArgsCount(0),
				instrument.WithResultCount(1), instrument.WithResultType(0, "bool")),
			Interceptor: "RWRTryLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*RWMutex", "RUnlock",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "RWRUnLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*RWMutex", "Lock",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "RWLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*RWMutex", "TryLock",
				instrument.WithArgsCount(0),
				instrument.WithResultCount(1), instrument.WithResultType(0, "bool")),
			Interceptor: "RWTryLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*RWMutex", "Unlock",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "RWUnLockInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*WaitGroup", "Add",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "int"),
				instrument.WithResultCount(0)),
			Interceptor: "WaitGroupAddInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*WaitGroup", "Done",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "WaitGroupDoneInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*WaitGroup", "Wait",
				instrument.WithArgsCount(0), instrument.WithResultCount(0)),
			Interceptor: "WaitGroupWaitInterceptor",
		},
	}
}

func (r *Instrument) FS() *embed.FS {
	return &fs
}
