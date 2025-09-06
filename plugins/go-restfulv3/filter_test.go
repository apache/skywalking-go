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

package restfulv3

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"

	"github.com/emicklei/go-restful/v3"

	"github.com/stretchr/testify/assert"

	agentv3 "github.com/apache/skywalking-go/protocols/skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func init() {
	core.ResetTracingContext()
}

func TestFilter(t *testing.T) {
	defer core.ResetTracingContext()
	container := restful.DefaultContainer
	ws := new(restful.WebService)
	ws.Route(ws.GET("/").To(func(request *restful.Request, response *restful.Response) {
		time.Sleep(2 * time.Millisecond)
		_, _ = response.Write([]byte("success"))
	}))
	container.Filter(filterInstance)
	container.Add(ws)

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest("GET", "/", http.NoBody)
	assert.Nil(t, err, "new request error should be nil")
	container.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusOK, recorder.Code, "http status code should be 200")

	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.NotNil(t, spans, "spans should not be nil")
	assert.Equal(t, 1, len(spans), "spans length should be 1")
	assert.Equal(t, agentv3.SpanType_Entry, spans[0].SpanType(), "span type should be entry")
	assert.Equal(t, "GET:/", spans[0].OperationName(), "operation name should be GET:/")
	assert.Nil(t, spans[0].Refs(), "refs should be nil")
	assert.Greater(t, spans[0].EndTime(), spans[0].StartTime(), "end time should be greater than start time")
}
