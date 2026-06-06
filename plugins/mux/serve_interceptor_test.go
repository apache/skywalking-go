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

package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// TestNonHijackerWriterIsUsable pins the nil-writer bug: an http.ResponseWriter
// that does not implement http.Hijacker (e.g. HTTP/2) took the default branch,
// which wrapped a nil writer - the first Write of the user handler then
// crashed with a nil dereference.
func TestNonHijackerWriterIsUsable(t *testing.T) {
	recorder := httptest.NewRecorder() // does not implement http.Hijacker

	rw := newResponseWriter(recorder)
	rw.WriteHeader(http.StatusNotFound)
	if _, err := rw.Write([]byte("not found")); err != nil {
		t.Fatal(err)
	}

	wrapped, ok := rw.(*writerWrapper)
	if !ok {
		t.Fatalf("non-hijacker writer must use the plain wrapper, got %T", rw)
	}
	if wrapped.statusCode != http.StatusNotFound {
		t.Fatalf("status code was not captured, got %d", wrapped.statusCode)
	}
	if recorder.Body.String() != "not found" {
		t.Fatalf("response body was not written through, got %q", recorder.Body.String())
	}
}

// TestServeInterceptorWithNonHijackerWriter runs the full interceptor pair on
// a non-hijacker writer and checks the reported span carries the status code.
func TestServeInterceptorWithNonHijackerWriter(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	request, err := http.NewRequest(http.MethodGet, "http://localhost/api/users", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()

	interceptor := &ServeHTTPInterceptor{}
	invocation := operator.NewInvocation(nil, recorder, request)
	if err := interceptor.BeforeInvoke(invocation); err != nil {
		t.Fatal(err)
	}

	// the user handler writes through the (previously nil) wrapped writer
	rw := invocation.Args()[0].(http.ResponseWriter)
	rw.WriteHeader(http.StatusCreated)

	if err := interceptor.AfterInvoke(invocation); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(core.GetReportedSpans()) < 1 {
		if time.Now().After(deadline) {
			t.Fatal("span was never reported")
		}
		time.Sleep(20 * time.Millisecond)
	}
	spans := core.GetReportedSpans()
	if len(spans) != 1 {
		t.Fatalf("expected one reported span, got %d", len(spans))
	}
	statusCode := ""
	for _, tag := range spans[0].Tags() {
		if tag.Key == tracing.TagStatusCode {
			statusCode = tag.Value
		}
	}
	if statusCode != "201" {
		t.Fatalf("status code tag mismatch: %q", statusCode)
	}
}
