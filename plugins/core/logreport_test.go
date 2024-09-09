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

package core

import (
	"fmt"
	"testing"

	"github.com/apache/skywalking-go/plugins/core/reporter"

	"github.com/stretchr/testify/assert"
)

type ExtractorWrapper struct {
	F func(headerKey string) (string, error)
}

func (e *ExtractorWrapper) Fun() func(headerKey string) (string, error) {
	return e.F
}

func TestGetLogContext(t *testing.T) {
	defer ResetTracingContext()
	serviceName := "test-service"
	serviceInstanceName := "test-instance"
	Tracing.ServiceEntity = &reporter.Entity{ServiceName: serviceName, ServiceInstanceName: serviceInstanceName}
	s, err := Tracing.CreateEntrySpan("/test", &ExtractorWrapper{
		F: func(headerKey string) (string, error) {
			return "", nil
		},
	})
	assert.Nil(t, err, "err should be nil")
	assert.NotNil(t, s, "span cannot be nil")
	context := Tracing.GetLogContext(true)
	assert.NotNil(t, context, "context cannot be nil")
	rootSpan, ok := s.(*RootSegmentSpan)
	assert.True(t, ok, "span should be root span")
	swCtx, ok := context.(*SkyWalkingLogContext)
	assert.True(t, ok)
	assert.NotNil(t, swCtx, "skywalkingContext cannot be nil")
	assert.Equal(t, serviceName, swCtx.ServiceName, "service name should be equal")
	assert.Equal(t, serviceInstanceName, serviceInstanceName, "service instance name should be equal")
	assert.Equal(t, "/test", swCtx.GetEndPointName(), "endpoint name should be equal")
	assert.Equal(t, rootSpan.Context().GetTraceID(), swCtx.TraceID, "trace id should be equal")
	assert.Equal(t, rootSpan.Context().GetSegmentID(), swCtx.TraceSegmentID, "trace segment id should be equal")
	assert.Equal(t, rootSpan.Context().GetSpanID(), swCtx.SpanID, "span id should be equal")
	assert.NotEqualf(t, "", swCtx.String(), "context string should not be empty")
	rootSpan.End()
}

func TestGetLogContextString(t *testing.T) {
	defer ResetTracingContext()
	s, err := Tracing.CreateLocalSpan("/test")
	assert.Nil(t, err, "err should be nil")
	assert.NotNil(t, s, "span cannot be nil")
	context := Tracing.GetLogContext(false)
	assert.NotNil(t, context, "context cannot be nil")
	stringCtx, ok := context.(fmt.Stringer)
	assert.True(t, ok)
	assert.NotNil(t, stringCtx, "stringCtx cannot be nil")
	assert.NotEqualf(t, "", stringCtx.String(), "context string should not be empty")
	rootSpan, ok := s.(*RootSegmentSpan)
	assert.True(t, ok, "span should be root span")
	rootSpan.End()
}
