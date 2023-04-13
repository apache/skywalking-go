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
	"time"

	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

// SpanType is used to identify entry, exit and local
type SpanType int32

// Tag are supported by sky-walking engine.
// As default, all Tags will be stored, but these ones have
// particular meanings.
type Tag string

const (
	// SpanTypeEntry is a entry span, eg http server
	SpanTypeEntry SpanType = 0
	// SpanTypeExit is a exit span, eg http client
	SpanTypeExit SpanType = 1
	// SpanTypeLocal is a local span, eg local method invoke
	SpanTypeLocal SpanType = 2
)

const (
	TagURL             Tag = "url"
	TagStatusCode      Tag = "status_code"
	TagHTTPMethod      Tag = "http.method"
	TagDBType          Tag = "db.type"
	TagDBInstance      Tag = "db.instance"
	TagDBStatement     Tag = "db.statement"
	TagDBSqlParameters Tag = "db.sql.parameters"
	TagMQQueue         Tag = "mq.queue"
	TagMQBroker        Tag = "mq.broker"
	TagMQTopic         Tag = "mq.topic"
)

// Span interface as commonv3 span specification
type Span interface {
	SetOperationName(string)
	GetOperationName() string
	SetPeer(string)
	GetPeer() string
	SetSpanLayer(agentv3.SpanLayer)
	GetSpanLayer() agentv3.SpanLayer
	SetComponent(int32)
	GetComponent() int32
	Tag(Tag, string)
	Log(time.Time, ...string)
	Error(time.Time, ...string)
	End()
	IsEntry() bool
	IsExit() bool
	IsValid() bool
	ParentSpan() Span
}
