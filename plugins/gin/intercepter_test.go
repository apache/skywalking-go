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

package gin

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	core.ResetTracingContext()
}

func TestInvoke(t *testing.T) {
	defer core.ResetTracingContext()
	interceptor := &ContextInterceptor{}
	request, err := http.NewRequest("GET", "http://localhost/skywalking/trace/f4dd2255-e3be-4636-b2e7-fc1d407a30d3", http.NoBody)
	assert.Nil(t, err, "new request error should be nil")
	c := &gin.Context{
		Request: request,
		Writer:  &testWriter{},
	}

	fullPath := reflect.ValueOf(c).Elem().FieldByName("fullPath")
	reflect.NewAt(fullPath.Type(), unsafe.Pointer(fullPath.UnsafeAddr())).Elem().Set(reflect.ValueOf("/skywalking/trace/:traceId"))

	invocation := operator.NewInvocation(c)
	err = interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err, "before invoke error should be nil")
	assert.NotNil(t, invocation.GetContext(), "context should not be nil")

	time.Sleep(100 * time.Millisecond)

	err = interceptor.AfterInvoke(invocation)
	assert.Nil(t, err, "after invoke error should be nil")

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.NotNil(t, spans, "spans should not be nil")
	assert.Equal(t, 1, len(spans), "spans length should be 1")
	assert.Equal(t, "GET:/skywalking/trace/:traceId", spans[0].OperationName(), "operation name should be GET:/skywalking/trace/:traceId")
	assert.Nil(t, spans[0].Refs(), "refs should be nil")
	assert.Greater(t, spans[0].EndTime(), spans[0].StartTime(), "end time should be greater than start time")
}

type testWriter struct {
	gin.ResponseWriter
}

func (i *testWriter) Status() int {
	return 200
}

func TestCollectHeaders(t *testing.T) {
	defer core.ResetTracingContext()

	config.CollectRequestHeaders = []string{"h1", "h2"}
	config.HeaderLengthThreshold = 17

	interceptor := &ContextInterceptor{}
	request, err := http.NewRequest("GET", "http://localhost/skywalking/trace", http.NoBody)
	assert.Nil(t, err, "new request error should be nil")
	request.Header.Set("h1", "h1-value")
	request.Header.Set("h2", "h2-value")

	c := &gin.Context{
		Request: request,
		Writer:  &testWriter{},
	}

	invocation := operator.NewInvocation(c)
	err = interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err, "before invoke error should be nil")
	assert.NotNil(t, invocation.GetContext(), "context should not be nil")

	time.Sleep(100 * time.Millisecond)

	err = interceptor.AfterInvoke(invocation)
	assert.Nil(t, err, "after invoke error should be nil")

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.Equal(t, 1, len(spans), "spans length should be 1")
	assert.Equal(t, 4, len(spans[0].Tags()), "tags length should be 4")

	index := 0
	for ; index < len(spans[0].Tags()); index++ {
		if spans[0].Tags()[index].Key == tracing.TagHTTPHeaders {
			break
		}
	}
	assert.Less(t, index, 4, "the index should be less than 4")
	assert.Equal(t, "h1=h1-value\nh2=h2", spans[0].Tags()[index].Value, "the tag Value should be h1=h1-value\nh2=h2-value")
}

type notFoundWriter struct {
	gin.ResponseWriter
}

func (i *notFoundWriter) Status() int {
	return http.StatusNotFound
}

func TestPathNotFoundInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	path := "/skywalking/trace/f4dd2255-e3be-4636-b2e7-fc1d407a30d3"
	interceptor := &ContextInterceptor{}
	request, err := http.NewRequest("GET", fmt.Sprintf("http://localhost%s", path), http.NoBody)
	assert.Nil(t, err, "new request error should be nil")

	c := &gin.Context{
		Request: request,
		Writer:  &notFoundWriter{},
	}

	invocation := operator.NewInvocation(c)
	err = interceptor.BeforeInvoke(invocation)
	assert.Nil(t, err, "before invoke error should be nil")
	assert.NotNil(t, invocation.GetContext(), "context should not be nil")

	time.Sleep(100 * time.Millisecond)

	err = interceptor.AfterInvoke(invocation)
	assert.Nil(t, err, "after invoke error should be nil")

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.NotNil(t, spans, "spans should not be nil")
	assert.Equal(t, 1, len(spans), "spans length should be 1")
	assert.Equal(t, fmt.Sprintf("GET:%s", path), spans[0].OperationName(), fmt.Sprintf("operation name should be GET:%s", path))
}
