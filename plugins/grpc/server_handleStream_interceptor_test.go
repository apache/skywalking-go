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

package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
)

func TestServerHandleStreamInterceptorBeforeInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &ServerHandleStreamInterceptor{}
	stream := &nativeStream{
		ctx:    context.Background(),
		method: "/api.Echo/UnaryEcho",
	}
	invocation := operator.NewInvocation(nil, nil, stream)

	err := interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err)
	assert.NotNil(t, invocation.GetContext())
}

func TestServerHandleStreamInterceptorAfterInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &ServerHandleStreamInterceptor{}
	stream := &nativeStream{
		ctx:    context.Background(),
		method: "/api.Echo/UnaryEcho",
	}
	invocation := operator.NewInvocation(nil, nil, stream)

	err := interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err)

	time.Sleep(100 * time.Millisecond)

	err = interceptor.AfterInvoke(invocation)
	assert.Nil(t, err)

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.NotNil(t, spans)
	assert.Equal(t, 1, len(spans))
	assert.Equal(t, "api.Echo.UnaryEcho", spans[0].OperationName())
}

func TestServerHandleStreamInterceptorAfterInvokeWithNilContext(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &ServerHandleStreamInterceptor{}
	invocation := operator.NewInvocation(nil, nil, nil)

	err := interceptor.AfterInvoke(invocation)
	assert.Nil(t, err)
}

func TestServerHandleStreamInterceptorV2BeforeInvokeWithServerStream(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &ServerHandleStreamInterceptorV2{}
	stream := &nativeServerStream{
		nativeStream: nativeStream{
			ctx:    context.Background(),
			method: "/api.Echo/ServerStreamingEcho",
		},
	}
	invocation := operator.NewInvocation(nil, nil, stream)

	err := interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err)
	assert.NotNil(t, invocation.GetContext())
}

func TestServerHandleStreamInterceptorV2AfterInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &ServerHandleStreamInterceptorV2{}
	stream := &nativeServerStream{
		nativeStream: nativeStream{
			ctx:    context.Background(),
			method: "/api.Echo/ServerStreamingEcho",
		},
	}
	invocation := operator.NewInvocation(nil, nil, stream)

	err := interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err)

	time.Sleep(100 * time.Millisecond)

	err = interceptor.AfterInvoke(invocation)
	assert.Nil(t, err)

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.NotNil(t, spans)
	assert.Equal(t, 1, len(spans))
	assert.Equal(t, "api.Echo.ServerStreamingEcho", spans[0].OperationName())
}

func TestServerHandleStreamInterceptorV2AfterInvokeWithNilContext(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &ServerHandleStreamInterceptorV2{}
	invocation := operator.NewInvocation(nil, nil, nil)

	err := interceptor.AfterInvoke(invocation)
	assert.Nil(t, err)
}
