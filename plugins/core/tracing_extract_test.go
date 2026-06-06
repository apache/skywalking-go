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
	"testing"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// secondHeader returns the sw8 headers of a second upstream segment, distinct
// from the package-level `header` built in tracing_test.go.
func secondUpstreamExtractor(correlation map[string]string) func(string) (string, error) {
	scx := SpanContext{
		Sample:                1,
		TraceID:               "2f2d4bf47bf711eab794acde48001122",
		ParentSegmentID:       "2e7c204a7bf711eab858acde48001122",
		ParentSpanID:          1,
		ParentService:         "service-2",
		ParentServiceInstance: "instance-2",
		ParentEndpoint:        "/producer/second",
		AddressUsedAtClient:   "mq.svc:9876",
		CorrelationContext:    correlation,
	}
	sw8 := scx.EncodeSW8()
	sw8Correlation := scx.EncodeSW8Correlation()
	return func(headerKey string) (string, error) {
		switch headerKey {
		case Header:
			return sw8, nil
		case HeaderCorrelation:
			return sw8Correlation, nil
		}
		return "", nil
	}
}

// TestExtractContextAddsRefs covers the batch-consumer flow: the entry span is
// created from the first message and every further message is attached as one
// more segment reference, with its correlation merged.
func TestExtractContextAddsRefs(t *testing.T) {
	ResetTracingContext()
	defer ResetTracingContext()

	entry, err := tracing.CreateEntrySpan("MQ/batch/Consumer", func(headerKey string) (string, error) {
		if headerKey == Header {
			return header, nil
		}
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := tracing.ExtractContext(secondUpstreamExtractor(map[string]string{"upstream": "second"})); err != nil {
		t.Fatalf("extract second upstream failed: %v", err)
	}
	// an empty/invalid carrier must be a silent no-op
	if err := tracing.ExtractContext(func(string) (string, error) { return "", nil }); err != nil {
		t.Fatalf("invalid carrier must not error: %v", err)
	}
	// correlation carried by the second message is visible on the segment
	if got := Tracing.GetCorrelationContextValue("upstream"); got != "second" {
		t.Fatalf("correlation was not merged, got %q", got)
	}

	entry.End()

	spans := waitReportedSpans(t, 1)
	if len(spans) != 1 {
		t.Fatalf("expected exactly one reported span, got %d", len(spans))
	}
	refs := spans[0].Refs()
	if len(refs) != 2 {
		t.Fatalf("expected 2 segment references (first message + extracted), got %d", len(refs))
	}
	if refs[0].GetTraceID() != traceID {
		t.Fatalf("first ref must keep the creation carrier, got %s", refs[0].GetTraceID())
	}
	if refs[1].GetTraceID() != "2f2d4bf47bf711eab794acde48001122" {
		t.Fatalf("second ref must carry the extracted upstream, got %s", refs[1].GetTraceID())
	}
}

// TestExtractContextRequiresEntrySpan pins the no-op behavior outside an entry
// span (mirroring the Java agent, only the EntrySpan carries extra refs).
func TestExtractContextRequiresEntrySpan(t *testing.T) {
	ResetTracingContext()
	defer ResetTracingContext()

	// no active span at all
	if err := tracing.ExtractContext(secondUpstreamExtractor(nil)); err != nil {
		t.Fatalf("no active span must be a no-op: %v", err)
	}

	local, err := tracing.CreateLocalSpan("local/op")
	if err != nil {
		t.Fatal(err)
	}
	if err := tracing.ExtractContext(secondUpstreamExtractor(nil)); err != nil {
		t.Fatalf("non-entry active span must be a no-op: %v", err)
	}
	local.End()

	spans := waitReportedSpans(t, 1)
	if got := len(spans[0].Refs()); got != 0 {
		t.Fatalf("local span must not gain refs, got %d", got)
	}
}
