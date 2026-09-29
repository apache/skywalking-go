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
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reporter

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
)

type captureAuthLogger struct {
	errors   []string
	warnings []string
}

func (c *captureAuthLogger) WithField(key string, value interface{}) interface{} { return c }
func (c *captureAuthLogger) Info(args ...interface{})                            {}
func (c *captureAuthLogger) Infof(format string, args ...interface{})            {}
func (c *captureAuthLogger) Warn(args ...interface{}) {
	c.warnings = append(c.warnings, fmt.Sprint(args...))
}
func (c *captureAuthLogger) Warnf(format string, args ...interface{}) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}
func (c *captureAuthLogger) Error(args ...interface{}) {
	c.errors = append(c.errors, "Error")
}
func (c *captureAuthLogger) Errorf(format string, args ...interface{}) {
	c.errors = append(c.errors, format)
}
func (c *captureAuthLogger) Debug(args ...interface{})                 {}
func (c *captureAuthLogger) Debugf(format string, args ...interface{}) {}

func TestAuthFailureLoggerRateLimits(t *testing.T) {
	log := &captureAuthLogger{}
	a := &authFailureLogger{logger: log}
	err := status.Error(codes.Unauthenticated, "bad token")
	a.note(err)
	a.note(err)
	if len(log.errors) != 1 {
		t.Fatalf("logs=%d, want 1", len(log.errors))
	}
	a.last = time.Now().Add(-authFailureLogInterval - time.Second)
	a.note(err)
	if len(log.errors) != 2 {
		t.Fatalf("logs=%d, want 2 after interval", len(log.errors))
	}
	a.note(status.Error(codes.Unavailable, "down"))
	if len(log.errors) != 2 {
		t.Fatalf("UNAVAILABLE should not auth-log, logs=%d", len(log.errors))
	}
	if !strings.Contains(log.errors[0], "authentication") {
		t.Fatalf("msg=%q", log.errors[0])
	}
}

func TestAuthFailureUnaryInterceptor(t *testing.T) {
	log := &captureAuthLogger{}
	a := &authFailureLogger{logger: log}
	interceptor := a.unaryInterceptor()
	err := interceptor(context.Background(), "/svc/Method", nil, nil, nil,
		func(ctx context.Context, method string, req, reply interface{},
			cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			return status.Error(codes.PermissionDenied, "nope")
		})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("err=%v", err)
	}
	if len(log.errors) != 1 {
		t.Fatalf("logs=%d", len(log.errors))
	}
}

type stubClientStream struct {
	grpc.ClientStream
	recvErr error
}

func (s *stubClientStream) RecvMsg(m interface{}) error { return s.recvErr }
func (s *stubClientStream) SendMsg(m interface{}) error { return nil }
func (s *stubClientStream) CloseSend() error            { return nil }
func (s *stubClientStream) Header() (metadata.MD, error) {
	return nil, nil
}
func (s *stubClientStream) Trailer() metadata.MD { return nil }
func (s *stubClientStream) Context() context.Context {
	return context.Background()
}

func TestAuthFailureStreamInterceptorNotesRecv(t *testing.T) {
	log := &captureAuthLogger{}
	a := &authFailureLogger{logger: log}
	interceptor := a.streamInterceptor()
	wrapped, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, "/svc/Method",
		func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
			method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return &stubClientStream{}, nil
		})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	recvErr := wrapped.RecvMsg(nil)
	if recvErr != nil {
		t.Fatalf("RecvMsg: %v", recvErr)
	}
	if len(log.errors) != 0 {
		t.Fatalf("unexpected logs=%v", log.errors)
	}
	authStream := wrapped.(*authWatchingClientStream)
	authStream.ClientStream = &stubClientStream{recvErr: status.Error(codes.Unauthenticated, "bad")}
	_ = wrapped.RecvMsg(nil)
	if len(log.errors) != 1 {
		t.Fatalf("logs=%d, want 1 after auth RecvMsg", len(log.errors))
	}
}

func TestConnectionManagerGetConnectionAfterClose(t *testing.T) {
	cm, err := NewConnectionManager(nil, time.Second, "127.0.0.1:9", "", nil)
	if err != nil {
		t.Fatalf("cm: %v", err)
	}
	if cm.authFailures != nil {
		t.Fatal("single-address manager must not allocate authFailureLogger")
	}
	if cm.IsMultiBackend() {
		t.Fatal("single-address must not be multi-backend")
	}
	// Close is a test helper that drops managed conns; historical GetConnection
	// has no closed gate and may dial again afterward.
	cm.Close()
}

func TestConnectionManagerAuthFailuresOnlyMulti(t *testing.T) {
	multi, err := NewConnectionManager(nil, time.Second, "10.0.0.1:9,10.0.0.2:9", "", nil)
	if err != nil {
		t.Fatalf("cm: %v", err)
	}
	defer multi.Close()
	if !multi.IsMultiBackend() || multi.authFailures == nil {
		t.Fatal("multi-address manager must be multi and allocate authFailureLogger")
	}
}

func TestIsMultiBackendServiceNormalize(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"127.0.0.1:11800", false},
		{"127.0.0.1:11800,", false},
		{"127.0.0.1:11800,127.0.0.1:11800", false},
		{"10.0.0.1:9,10.0.0.2:9", true},
		{"a.example.com:11800, b.example.com:11800", true},
	}
	for _, tc := range cases {
		cm, err := NewConnectionManager(nil, time.Second, tc.raw, "", nil)
		if err != nil {
			t.Fatalf("cm %q: %v", tc.raw, err)
		}
		if cm.IsMultiBackend() != tc.want {
			t.Fatalf("IsMultiBackend(%q)=%v want %v", tc.raw, cm.IsMultiBackend(), tc.want)
		}
		cm.Close()
	}
}

func TestBackendStreamContextStopOpenClearsTimeout(t *testing.T) {
	ctx, cancel, stopOpen := BackendStreamContext(time.Second)
	defer cancel()
	if stopOpen() {
		t.Fatal("immediate stopOpen must not report timedOut")
	}
	_ = ctx
}

func TestBackendStreamContextReportsTimedOut(t *testing.T) {
	ctx, cancel, stopOpen := BackendStreamContext(time.Millisecond)
	defer cancel()
	cancel()
	if !stopOpen() {
		t.Fatal("stopOpen must report timedOut when ctx already canceled")
	}
	if ctx.Err() == nil {
		t.Fatal("expected canceled ctx")
	}
}

func TestBackendRPCContextMultiHasDeadline(t *testing.T) {
	ctx, cancel := BackendRPCContext(time.Second)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("BackendRPCContext must set a deadline")
	}
}

func TestConnectionManagerResolvedBackendAddresses(t *testing.T) {
	var published []string
	builder, err := newStaticBackendResolverBuilder(nil, []string{"10.0.0.1:11800", "10.0.0.2:11800"},
		func(addrs []string) { published = append([]string(nil), addrs...) })
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	cc := &testResolverClientConn{}
	r, err := builder.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer r.Close()
	if len(published) != 2 {
		t.Fatalf("published=%v", published)
	}

	cm, err := NewConnectionManager(nil, time.Second, "10.0.0.1:9,10.0.0.2:9", "", nil)
	if err != nil {
		t.Fatalf("cm: %v", err)
	}
	defer cm.Close()
	const resolvedA = "10.0.0.1:9"
	cm.storeResolvedBackendAddresses([]string{resolvedA, "10.0.0.2:9"})
	got := resolvedBackendAddressesForTest(cm)
	if len(got) != 2 || got[0] != resolvedA {
		t.Fatalf("got %#v", got)
	}
	got[0] = "mutated"
	if resolvedBackendAddressesForTest(cm)[0] != resolvedA {
		t.Fatal("resolvedBackendAddressesForTest must return a copy")
	}
}
