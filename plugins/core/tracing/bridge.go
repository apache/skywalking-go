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

type AsyncSpan interface {
	// PrepareAsync the span finished at current tracing context, but current span is still alive until AsyncFinish called
	PrepareAsync()
	// AsyncFinish to finished current async span
	AsyncFinish()
}

// AdaptSpan for adapt with agent core
type AdaptSpan interface {
	AsyncSpan

	GetTraceID() string
	GetSegmentID() string
	GetSpanID() int32
	SetOperationName(string)
	SetPeer(string)
	SetSpanLayer(int32)
	SetComponent(int32)
	Tag(string, string)
	Log(...string)
	Error(...string)
	End()
}

type SpanWrapper struct {
	Span AdaptSpan
}

func newSpanAdapter(s AdaptSpan) Span {
	return &SpanWrapper{Span: s}
}

func (s *SpanWrapper) TraceID() string {
	return s.Span.GetTraceID()
}

func (s *SpanWrapper) TraceSegmentID() string {
	return s.Span.GetSegmentID()
}

func (s *SpanWrapper) SpanID() int32 {
	return s.Span.GetSpanID()
}

func (s *SpanWrapper) Tag(k, v string) {
	s.Span.Tag(k, v)
}

func (s *SpanWrapper) SetSpanLayer(l int32) {
	s.Span.SetSpanLayer(l)
}

func (s *SpanWrapper) SetOperationName(name string) {
	s.Span.SetOperationName(name)
}

func (s *SpanWrapper) SetPeer(v string) {
	s.Span.SetPeer(v)
}

func (s *SpanWrapper) Log(v ...string) {
	s.Span.Log(v...)
}

func (s *SpanWrapper) SetComponent(v int32) {
	s.Span.SetComponent(v)
}

func (s *SpanWrapper) Error(v ...string) {
	s.Span.Error(v...)
}

func (s *SpanWrapper) End() {
	s.Span.End()
}

func (s *SpanWrapper) PrepareAsync() {
	s.Span.PrepareAsync()
}

func (s *SpanWrapper) AsyncFinish() {
	s.Span.AsyncFinish()
}
