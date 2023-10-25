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

package main

import (
	"net/http"
	"strconv"

	"github.com/apache/skywalking-go/toolkit/trace"
)

func testTag() {
	trace.CreateLocalSpan("testSetTag")
	trace.SetTag("SetTag", "success")
	trace.StopSpan()
}

func testLog() {
	trace.CreateLocalSpan("testAddLog")
	trace.AddLog("AddLog", "success")
	trace.StopSpan()
}

func testSetOperationName() {
	trace.CreateLocalSpan("testSetOperationName_failed")
	trace.SetOperationName("testSetOperationName_success")
	trace.StopSpan()
}

func testGetTraceID() {
	trace.CreateLocalSpan("testGetTraceID")
	trace.SetTag("traceID", trace.GetTraceID())
	trace.StopSpan()
}

func testGetSpanID() {
	trace.CreateLocalSpan("testGetSpanID")
	trace.SetTag("spanID", strconv.FormatInt(int64(trace.GetSpanID()), 10))
	trace.StopSpan()
}

func testGetSegmentID() {
	trace.CreateLocalSpan("testGetSegmentID")
	trace.SetTag("segmentID", trace.GetSegmentID())
	trace.StopSpan()
}

func testContext() {
	trace.CreateLocalSpan("testCaptureContext")
	captureSpanID := trace.GetSpanID()
	ctx := trace.CaptureContext()
	trace.StopSpan()

	trace.ContinueContext(ctx)
	continueSpanID := trace.GetSpanID()
	trace.CreateLocalSpan("testContinueContext")
	if captureSpanID == continueSpanID {
		trace.SetTag("testContinueContext", "success")
	}
	trace.StopSpan()
}

func testContextCarrier() {
	request, _ := http.NewRequest("GET", "http://localhost/", http.NoBody)
	trace.CreateExitSpan("ExitSpan", request.Host, func(headerKey, headerValue string) error {
		request.Header.Add(headerKey, headerValue)
		return nil
	})

	trace.CreateEntrySpan("EntrySpan", func(headerKey string) (string, error) {
		return request.Header.Get(headerKey), nil
	})
	trace.StopSpan()

	trace.StopSpan()
}

func testCorrelation() {
	trace.SetCorrelation("testCorrelation", "success")
	_, err := http.Get("http://localhost:8080/provider")
	if err != nil {
		return
	}
}

func testComponent() {
	trace.CreateLocalSpan("testComponent")
	trace.SetComponent(5006)
	trace.StopSpan()
}

func testAsyncInCrossGoroutine() {
	var ch = make(chan string)
	s, _ := trace.CreateLocalSpan("testAsyncInCrossGoroutine")
	s.PrepareAsync()
	trace.StopSpan()
	go func() {
		s.SetTag("testAsyncTag", "success")
		s.AddLog("testAsyncLog", "success")
		s.AsyncFinish()
		ch <- ""
	}()
	<-ch
}
