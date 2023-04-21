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

package core

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/reporter"
)

type CorrelationConfig struct {
	MaxKeyCount  int
	MaxValueSize int
}

type Tracer struct {
	Service  string
	Instance string
	Reporter reporter.Reporter
	// 0 not init 1 init
	initFlag int32
	Sampler  Sampler
	Log      operator.LogOperator
	// correlation *CorrelationConfig	// temporarily disable, because haven't been implemented yet
	cdsWatchers []reporter.AgentConfigChangeWatcher
}

func (t *Tracer) InitSuccess() bool {
	return t.initFlag == 1
}
