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

package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go-micro.dev/v4/client"
	"go-micro.dev/v4/codec"
	"go-micro.dev/v4/metadata"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/reporter"
)

type fakeRequest struct {
	service  string
	endpoint string
}

func (r *fakeRequest) Service() string     { return r.service }
func (r *fakeRequest) Method() string      { return r.endpoint }
func (r *fakeRequest) Endpoint() string    { return r.endpoint }
func (r *fakeRequest) ContentType() string { return "application/protobuf" }
func (r *fakeRequest) Body() interface{}   { return nil }
func (r *fakeRequest) Codec() codec.Writer { return nil }
func (r *fakeRequest) Stream() bool        { return false }

type fakeMessage struct {
	topic       string
	contentType string
}

func (m *fakeMessage) Topic() string        { return m.topic }
func (m *fakeMessage) Payload() interface{} { return nil }
func (m *fakeMessage) ContentType() string  { return m.contentType }

// fakeClient stands in for the go-micro client below the wrapper and records
// the calls that reach it.
type fakeClient struct {
	client.Client
	calls     int
	streams   int
	publishes int
	md        metadata.Metadata
}

func (f *fakeClient) Call(ctx context.Context, _ client.Request, _ interface{}, _ ...client.CallOption) error {
	f.calls++
	f.md, _ = metadata.FromContext(ctx)
	return nil
}

func (f *fakeClient) Stream(ctx context.Context, _ client.Request, _ ...client.CallOption) (client.Stream, error) {
	f.streams++
	f.md, _ = metadata.FromContext(ctx)
	return nil, nil
}

func (f *fakeClient) Publish(ctx context.Context, _ client.Message, _ ...client.PublishOption) error {
	f.publishes++
	f.md, _ = metadata.FromContext(ctx)
	return nil
}

// recordingLogger keeps the agent warnings, because the test tracer has no
// logger.
type recordingLogger struct {
	mu       sync.Mutex
	warnings []string
}

func (l *recordingLogger) WithField(string, interface{}) interface{} { return l }
func (l *recordingLogger) Info(...interface{})                       {}
func (l *recordingLogger) Infof(string, ...interface{})              {}
func (l *recordingLogger) Warn(args ...interface{})                  { l.add(fmt.Sprint(args...)) }
func (l *recordingLogger) Warnf(format string, args ...interface{}) {
	l.add(fmt.Sprintf(format, args...))
}
func (l *recordingLogger) Error(...interface{})          {}
func (l *recordingLogger) Errorf(string, ...interface{}) {}

func (l *recordingLogger) add(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warnings = append(l.warnings, msg)
}

func (l *recordingLogger) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.warnings...)
}

// TestUntraceableCallsAreDelegated covers calls for which no exit span can be
// created: go-micro serves a request with an empty service name when the
// address is given with client.WithAddress, and a message may have no content
// type. The wrapper must still run such calls, untraced.
func TestUntraceableCallsAreDelegated(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()
	logger := &recordingLogger{}
	core.Tracing.Log = &core.LogWrapper{Logger: logger}

	fake := &fakeClient{}
	wrapper := NewClientWrapper(fake)
	req := &fakeRequest{endpoint: "Say.Hello"}

	if err := wrapper.Call(context.Background(), req, nil, client.WithAddress("127.0.0.1:8888")); err != nil {
		t.Errorf("Call returned the tracing error: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("Call reached the client %d times, want 1", fake.calls)
	}

	if _, err := wrapper.Stream(context.Background(), req, client.WithAddress("127.0.0.1:8888")); err != nil {
		t.Errorf("Stream returned the tracing error: %v", err)
	}
	if fake.streams != 1 {
		t.Errorf("Stream reached the client %d times, want 1", fake.streams)
	}

	if err := wrapper.Publish(context.Background(), &fakeMessage{topic: "events"}); err != nil {
		t.Errorf("Publish returned the tracing error: %v", err)
	}
	if fake.publishes != 1 {
		t.Errorf("Publish reached the client %d times, want 1", fake.publishes)
	}

	time.Sleep(100 * time.Millisecond)
	if spans := core.GetReportedSpans(); len(spans) != 0 {
		t.Errorf("untraceable calls reported %d spans, want 0", len(spans))
	}
	warnings := logger.all()
	if len(warnings) != 3 {
		t.Errorf("logged %d warnings, want one per call: %q", len(warnings), warnings)
	}
	for _, w := range warnings {
		if !strings.Contains(w, "peer is empty") {
			t.Errorf("unexpected warning: %q", w)
		}
	}
}

// TestCallIsTraced keeps the regular path covered: an exit span with the
// service as peer, and the context propagated in the metadata.
func TestCallIsTraced(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	fake := &fakeClient{}
	wrapper := NewClientWrapper(fake)
	req := &fakeRequest{service: "go.micro.srv.greeter", endpoint: "Say.Hello"}

	if err := wrapper.Call(context.Background(), req, nil); err != nil {
		t.Fatalf("Call returned an error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("Call reached the client %d times, want 1", fake.calls)
	}
	if _, ok := fake.md.Get("sw8"); !ok {
		t.Errorf("sw8 is missing from the outgoing metadata: %v", fake.md)
	}

	spans := waitForSpans(1)
	if len(spans) != 1 {
		t.Fatalf("reported %d spans, want 1", len(spans))
	}
	if spans[0].OperationName() != "go.micro.srv.greeter.Say.Hello" {
		t.Errorf("operation name = %q", spans[0].OperationName())
	}
	if spans[0].Peer() != "go.micro.srv.greeter" {
		t.Errorf("peer = %q", spans[0].Peer())
	}
	if spans[0].ComponentID() != 5008 {
		t.Errorf("component = %d", spans[0].ComponentID())
	}
}

func waitForSpans(count int) []reporter.ReportedSpan {
	deadline := time.Now().Add(2 * time.Second)
	for {
		spans := core.GetReportedSpans()
		if len(spans) >= count || time.Now().After(deadline) {
			return spans
		}
		time.Sleep(10 * time.Millisecond)
	}
}
