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
	SpanLayerDatabase     SpanLayer = 1
	SpanLayerRPCFramework SpanLayer = 2
	SpanLayerHTTP         SpanLayer = 3
	SpanLayerMQ           SpanLayer = 4
	SpanLayerCache        SpanLayer = 5
	SpanLayerFAAS         SpanLayer = 6
)

// Tag are supported by sky-walking engine.
// As default, all Tags will be stored, but these ones have
// particular meanings.
type Tag string

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

// WithLayer set the SpanLayer of the Span
func WithLayer(layer SpanLayer) SpanOption {
	return buildSpanOption(func(s AdaptSpan) {
		s.SetSpanLayer(int32(layer))
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
	// Tag set the Tag of the Span
	Tag(Tag, string)
	// SetSpanLayer set the SpanLayer of the Span
	SetSpanLayer(SpanLayer)
	// SetOperationName re-set the operation name of the Span
	SetOperationName(string)
	// SetPeer re-set the peer address of the Span
	SetPeer(string)
	// Log add log to the Span
	Log(...string)
	// Error add error log to the Span
	Error(...string)
	// End end the Span
	End()
}
