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
	"time"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// The span reuse rule in CreateEntrySpan/CreateExitSpan returns the existing
// active span when the span types match, so one span can have several logical
// owners that each call End once. These tests pin the required semantics: the
// span is frozen and reported only by the LAST End, so writes from the outer
// owner that happen after the inner owner's End still land (e.g. gorm tags
// db.statement after the sql driver plugin already ended the reused span).

func waitReportedSpans(t *testing.T, want int) []reporter.ReportedSpan {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		spans := GetReportedSpans()
		if len(spans) >= want {
			return spans
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d reported spans, got %d", want, len(spans))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func findReportedSpan(spans []reporter.ReportedSpan, name string) reporter.ReportedSpan {
	for _, s := range spans {
		if s.OperationName() == name {
			return s
		}
	}
	return nil
}

func reportedTagValue(s reporter.ReportedSpan, key string) (string, bool) {
	for _, tag := range s.Tags() {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

// TestExitSpanReuseFreezesOnLastEnd replicates the gorm + sql driver timeline
// from the gorm-postgres plugin scenario: gorm creates the exit span, the sql
// driver's CreateExitSpan reuses it and Ends it first, and only afterwards
// gorm tags db.statement and Ends. The tag must not be dropped and the span
// must be reported exactly once.
func TestExitSpanReuseFreezesOnLastEnd(t *testing.T) {
	ResetTracingContext()
	defer ResetTracingContext()

	entry, err := tracing.CreateEntrySpan("GET:/execute", func(string) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}
	outer, err := tracing.CreateExitSpan("users/create", "db:5432", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	inner, err := tracing.CreateExitSpan("PostgreSQL/Exec", "db:5432", func(k, v string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}

	inner.End()                                                   // inner owner: must NOT freeze the reused span
	outer.Tag("db.statement", "INSERT INTO users VALUES ($1,$2)") // late outer write must land
	outer.End()                                                   // last owner: freezes and reports
	entry.End()

	spans := waitReportedSpans(t, 2)
	if len(spans) != 2 {
		t.Fatalf("reuse must not create or report an extra span, got %d", len(spans))
	}
	exitSpan := findReportedSpan(spans, "users/create")
	if exitSpan == nil {
		t.Fatal("exit span was not reported (or was renamed by the reuse)")
	}
	if v, ok := reportedTagValue(exitSpan, "db.statement"); !ok || v != "INSERT INTO users VALUES ($1,$2)" {
		t.Fatalf("tag written after the inner owner's End was dropped, tags=%v", exitSpan.Tags())
	}
	if exitSpan.EndTime() <= 0 {
		t.Fatal("reported reused span has no end time")
	}
}

// TestEntrySpanReuseFreezesOnLastEnd is the entry-side twin (e.g. an http
// framework plugin reusing the net/http entry span): the inner owner renames
// and Ends first, the outer owner then tags the status code and Ends.
func TestEntrySpanReuseFreezesOnLastEnd(t *testing.T) {
	ResetTracingContext()
	defer ResetTracingContext()

	outer, err := tracing.CreateEntrySpan("GET:/raw", func(string) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}
	inner, err := tracing.CreateEntrySpan("GET:/renamed", func(string) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}

	inner.End()                     // inner owner: must NOT freeze the reused span
	outer.Tag("status_code", "200") // late outer write must land
	outer.End()                     // last owner: freezes and reports the root segment

	spans := waitReportedSpans(t, 1)
	if len(spans) != 1 {
		t.Fatalf("entry reuse must report exactly one span, got %d", len(spans))
	}
	if spans[0].OperationName() != "GET:/renamed" {
		t.Fatalf("reuse must keep the inner owner's rename, got %s", spans[0].OperationName())
	}
	if v, ok := reportedTagValue(spans[0], "status_code"); !ok || v != "200" {
		t.Fatalf("tag written after the inner owner's End was dropped, tags=%v", spans[0].Tags())
	}
}
