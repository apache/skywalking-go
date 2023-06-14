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

package tracing

// Extractor is a tool specification which define how to
// extract trace parent context from propagation context
type Extractor func(headerKey string) (string, error)

// Injector is a tool specification which define how to
// inject trace context into propagation context
type Injector func(headerKey, headerValue string) error

// SpanOption allows for functional options to adjust behavior
// of a AdaptSpan to be created by CreateLocalSpan
type SpanOption interface {
	Apply(interface{})
}

// SpanLayer define the Span belong to which layer
type SpanLayer int32

var (
	SpanLayerDatabase     int32 = 1
	SpanLayerRPCFramework int32 = 2
	SpanLayerHTTP         int32 = 3
	SpanLayerMQ           int32 = 4
	SpanLayerCache        int32 = 5
	SpanLayerFAAS         int32 = 6
)

// Tag are supported by sky-walking engine.
// As default, all Tags will be stored, but these ones have
// particular meanings.
type Tag string

const (
	TagURL             = "url"
	TagStatusCode      = "status_code"
	TagHTTPMethod      = "http.method"
	TagHTTPParams      = "http.params"
	TagDBType          = "db.type"
	TagDBInstance      = "db.instance"
	TagDBStatement     = "db.statement"
	TagDBSqlParameters = "db.sql.parameters"
	TagMQQueue         = "mq.queue"
	TagMQBroker        = "mq.broker"
	TagMQTopic         = "mq.topic"
	TagCacheType       = "cache.type"
	TagCacheOp         = "cache.op"
	TagCacheCmd        = "cache.cmd"
	TagCacheKey        = "cache.key"
	TagCacheArgs       = "cache.args"
)

// WithLayer set the SpanLayer of the Span
func WithLayer(layer int32) SpanOption {
	return buildSpanOption(func(s AdaptSpan) {
		s.SetSpanLayer(layer)
	})
}

// WithComponent set the component id of the Span
func WithComponent(componentID int32) SpanOption {
	return buildSpanOption(func(s AdaptSpan) {
		s.SetComponent(componentID)
	})
}

// WithTag set the Tag of the Span
func WithTag(key Tag, value string) SpanOption {
	return buildSpanOption(func(s AdaptSpan) {
		s.Tag(string(key), value)
	})
}

type spanOpImpl struct {
	exe func(s AdaptSpan)
}

func (s *spanOpImpl) Apply(span interface{}) {
	s.exe(span.(AdaptSpan))
}

func buildSpanOption(e func(s AdaptSpan)) SpanOption {
	return &spanOpImpl{exe: e}
}

type ExtractorWrapper interface {
	Fun() func(headerKey string) (string, error)
}

type InjectorWrapper interface {
	Fun() func(headerKey, headerValue string) error
}

// Span for plugin API
type Span interface {
	// AsyncSpan Async API
	AsyncSpan

	// TraceID of span
	TraceID() string
	// TraceSegmentID current segment ID of span
	TraceSegmentID() string
	// SpanID of span
	SpanID() int32

	// Tag set the Tag of the Span
	Tag(string, string)
	// SetSpanLayer set the SpanLayer of the Span
	SetSpanLayer(int32)
	// SetOperationName re-set the operation name of the Span
	SetOperationName(string)
	// SetPeer re-set the peer address of the Span
	SetPeer(string)
	// SetComponent re-set the component id of the Span
	SetComponent(int32)
	// Log add log to the Span
	Log(...string)
	// Error add error log to the Span
	Error(...string)
	// End end the Span
	End()
}
