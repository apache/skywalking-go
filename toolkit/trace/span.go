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

package trace

type EventType string

const (
	// DebugEventType Indicates the event type is "debug"
	DebugEventType EventType = "debug"

	// InfoEventType Indicates the event type is "info"
	InfoEventType EventType = "info"

	// WarnEventType Indicates the event type is "warn"
	WarnEventType EventType = "warn"

	// ErrorEventType Indicates the event type is "error"
	ErrorEventType EventType = "error"
)

func (*SpanRef) PrepareAsync() {
}

func (*SpanRef) AsyncFinish() {
}

// nolint
func (*SpanRef) SetTag(key string, value string) {
}

func (*SpanRef) AddLog(...string) {
}

// AddEvent Add an event of the specified type to SpanRef.
func (*SpanRef) AddEvent(et EventType, event string) {
}
