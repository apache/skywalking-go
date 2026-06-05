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
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	e2eChildEnv    = "SW_SPAN_E2E_CHILD"
	e2eDurationEnv = "SW_SPAN_E2E_DURATION"
	e2eOKMarker    = "E2E_OK"
)

// TestE2ESpanCrashSafety is the end-to-end guarantee for apache/skywalking#13885:
// under every concurrency-abuse pattern found in the audit, the agent must
// produce NO panic and NO runtime fatal error (`runtime.throw`, e.g. "invalid
// pointer found on stack" / "concurrent map writes").
//
// A runtime.throw is unrecoverable and kills the process, so this cannot be
// asserted in-process: the hostile workload runs in a CHILD test process and
// the parent asserts a clean exit. Any panic, data-race fatal, GC/stack-scan
// "bad pointer" throw or deadlock (child test timeout) fails this test.
//
// Tune the stress duration with SW_SPAN_E2E_DURATION (default 3s).
func TestE2ESpanCrashSafety(t *testing.T) {
	if os.Getenv(e2eChildEnv) == "1" {
		runE2EChild()
		return
	}
	if testing.Short() {
		t.Skip("skipping subprocess e2e in -short mode")
	}

	cmd := exec.Command(os.Args[0],
		"-test.run", "^TestE2ESpanCrashSafety$", "-test.v", "-test.timeout", "120s")
	// Runtime hardening for the child: clobberfree (Go 1.13+, harmlessly
	// ignored by runtimes without it) makes the GC overwrite freed objects
	// with junk, so any use-after-free (the silent precursor of the production
	// "invalid pointer found on stack" throw) crashes immediately and
	// deterministically instead of depending on scheduling luck.
	// invalidptr is on by default and kept explicit for documentation.
	godebug := "clobberfree=1,invalidptr=1"
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GODEBUG=") {
			// keep our hardening LAST: runtime parsegodebug assigns each
			// key=value as encountered front-to-back, so the last occurrence
			// of a duplicated key wins (verified against runtime1.go)
			godebug = strings.TrimPrefix(kv, "GODEBUG=") + "," + godebug
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env, e2eChildEnv+"=1", "GODEBUG="+godebug)
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		// non-zero exit = panic, runtime.throw or test timeout in the child
		t.Fatalf("hostile-workload child process did not exit cleanly (panic or runtime fatal): %v\n--- child output ---\n%s", err, output)
	}
	for _, fatal := range []string{"fatal error:", "panic:", "DATA RACE"} {
		if strings.Contains(output, fatal) {
			t.Fatalf("child output contains %q:\n--- child output ---\n%s", fatal, output)
		}
	}
	if !strings.Contains(output, e2eOKMarker) {
		t.Fatalf("child never reached the completion marker:\n--- child output ---\n%s", output)
	}

	// sanity: the pipeline must have actually transformed and marshalled work,
	// otherwise the e2e silently tested nothing
	segments := parseE2ECounter(t, output, "segments")
	marshals := parseE2ECounter(t, output, "marshals")
	if segments == 0 || marshals == 0 {
		t.Fatalf("e2e processed no data (segments=%d marshals=%d):\n%s", segments, marshals, output)
	}
	t.Logf("e2e clean: segments=%d marshals=%d", segments, marshals)
}

func runE2EChild() {
	d := 3 * time.Second
	if v := os.Getenv(e2eDurationEnv); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil && parsed > 0 {
			d = parsed
		}
	}
	segments, marshals := runHostileSpanWorkload(d)
	fmt.Printf("%s segments=%d marshals=%d\n", e2eOKMarker, segments, marshals)
}

func parseE2ECounter(t *testing.T, output, name string) int64 {
	t.Helper()
	idx := strings.Index(output, name+"=")
	if idx < 0 {
		t.Fatalf("counter %q missing from child output", name)
	}
	rest := output[idx+len(name)+1:]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		t.Fatalf("counter %q has no digits after '='", name)
	}
	v, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil {
		t.Fatalf("counter %q unparsable: %v", name, err)
	}
	return v
}
