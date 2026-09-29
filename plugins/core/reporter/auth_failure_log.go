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
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

const authFailureLogInterval = 30 * time.Second

// authFailureLogger rate-limits UNAUTHENTICATED / PERMISSION_DENIED logs so a
// bad credential does not spam (and does not encourage treating auth as failover).
type authFailureLogger struct {
	logger operator.LogOperator
	mu     sync.Mutex
	last   time.Time
}

func (a *authFailureLogger) note(err error) {
	if a == nil || a.logger == nil || err == nil {
		return
	}
	st, ok := status.FromError(err)
	if !ok {
		return
	}
	switch st.Code() {
	case codes.Unauthenticated, codes.PermissionDenied:
	default:
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if !a.last.IsZero() && now.Sub(a.last) < authFailureLogInterval {
		return
	}
	a.last = now
	a.logger.Errorf("backend rejected authentication/authorization (%s); "+
		"not treating this as multi-backend failover — check reporter.grpc.authentication",
		st.Code())
}

func (a *authFailureLogger) unaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		a.note(err)
		return err
	}
}

func (a *authFailureLogger) streamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, opts...)
		a.note(err)
		if err != nil || stream == nil {
			return stream, err
		}
		return &authWatchingClientStream{ClientStream: stream, auth: a}, nil
	}
}

// authWatchingClientStream notes auth failures on Recv/Send, not only stream open.
type authWatchingClientStream struct {
	grpc.ClientStream
	auth *authFailureLogger
}

func (s *authWatchingClientStream) SendMsg(m interface{}) error {
	err := s.ClientStream.SendMsg(m)
	s.auth.note(err)
	return err
}

func (s *authWatchingClientStream) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	s.auth.note(err)
	return err
}

func (s *authWatchingClientStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	s.auth.note(err)
	return err
}

func (s *authWatchingClientStream) Header() (metadata.MD, error) {
	md, err := s.ClientStream.Header()
	s.auth.note(err)
	return md, err
}
