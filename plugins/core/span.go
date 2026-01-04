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
	"github.com/apache/skywalking-go/plugins/core/tracing"

	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
)

// SpanType is used to identify entry, exit and local
type SpanType int32

const (
	// SpanTypeEntry is a entry span, eg http server
	SpanTypeEntry SpanType = 0
	// SpanTypeExit is a exit span, eg http client
	SpanTypeExit SpanType = 1
	// SpanTypeLocal is a local span, eg local method invoke
	SpanTypeLocal SpanType = 2
)

// TracingSpan interface as commonv3 span specification
type TracingSpan interface {
	tracing.AdaptSpan
	SetOperationName(string)
	GetOperationName() string
	SetPeer(string)
	GetPeer() string
	GetSpanLayer() agentv3.SpanLayer
	SetComponent(int32)
	GetComponent() int32
	End()
	IsEntry() bool
	IsExit() bool
	IsValid() bool
	ParentSpan() TracingSpan
	IsProfileTarget() bool
}
