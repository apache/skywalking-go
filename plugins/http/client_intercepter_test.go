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

package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"

	"github.com/stretchr/testify/assert"
)

func init() {
	core.ResetTracingContext()
}

func TestClientInvoke(t *testing.T) {
	defer core.ResetTracingContext()
	interceptor := &ClientInterceptor{}
	request, err := http.NewRequest("GET", "http://localhost/", http.NoBody)
	assert.Nil(t, err, "new request error should be nil")
	invocation := operator.NewInvocation(nil, request)
	err = interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err, "before invoke error should be nil")
	assert.NotNil(t, invocation.GetContext(), "context should not be nil")

	time.Sleep(100 * time.Millisecond)

	err = interceptor.AfterInvoke(invocation, &http.Response{
		StatusCode: 200,
	}, nil)
	assert.Nil(t, err, "after invoke error should be nil")

	time.Sleep(100 * time.Millisecond)

	reqWithoutHost, err := http.NewRequest("GET", "/api/hello", http.NoBody) // Create request with path only, will result empty Request.Host
	// Set URL after is valid behavior
	reqWithoutHost.URL.Scheme = "https"
	reqWithoutHost.URL.Host = "localhost"
	assert.Nil(t, err, "new request with no host in url should be no error")
	invocationWithoutHost := operator.NewInvocation(nil, reqWithoutHost)
	err = interceptor.BeforeInvoke(invocationWithoutHost)
	assert.Nil(t, err, "BeforeInvoke with URL.Host should be no error")

	time.Sleep(100 * time.Millisecond)

	spans := core.GetReportedSpans()
	assert.NotNil(t, spans, "spans should not be nil")
	assert.Equal(t, 1, len(spans), "spans length should be 1")
	assert.Equal(t, "GET:/", spans[0].OperationName(), "operation name should be GET:/")
	assert.Nil(t, spans[0].Refs(), "refs should be nil")
	assert.Greater(t, spans[0].EndTime(), spans[0].StartTime(), "end time should be greater than start time")
}

func TestClientInvokeError(t *testing.T) {
	defer core.ResetTracingContext()
	interceptor := &ClientInterceptor{}
	request, err := http.NewRequest("GET", "http://localhost/", http.NoBody)
	assert.Nil(t, err, "new request error should be nil")
	invocation := operator.NewInvocation(nil, request)
	err = interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err, "before invoke error should be nil")
	assert.NotNil(t, invocation.GetContext(), "context should not be nil")

	time.Sleep(100 * time.Millisecond)

	err = interceptor.AfterInvoke(invocation, &http.Response{
		StatusCode: 500,
	}, nil)
	assert.Nil(t, err, "after invoke error should be nil")

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.NotNil(t, spans, "spans should not be nil")
	assert.Equal(t, 1, len(spans), "spans length should be 1")
	assert.True(t, spans[0].IsError(), "span should be error")
}
