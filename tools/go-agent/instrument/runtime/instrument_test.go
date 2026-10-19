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

package runtime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
)

// The shape of the runtime functions that create goroutines.
const goroutineCreationSource = `package runtime

type g struct {
	goid uint64
}

func newproc(fn *funcval) {
	gp := getg()
	systemstack(func() {
		newg := newproc1(fn, gp, 0, false, 0)
		runqput(newg)
	})
}

func newproc1(fn *funcval, callergp *g, callerpc uintptr, parked bool, waitreason waitReason) *g {
	newg := gfget()
	return newg
}

func newcoro(f func(*coro)) *coro {
	c := new(coro)
	gp := getg()
	systemstack(func() {
		gp = newproc1(nil, gp, 0, true, 0)
	})
	return c
}
`

// newproc1 runs on the system stack, where blocking on a contended lock is a
// fatal error, so the context snapshot must be taken by its callers before
// they switch to the system stack, and newproc1 must only hand it over.
func TestGoroutineContextSnapshotOutsideSystemStack(t *testing.T) {
	file, err := decorator.Parse(goroutineCreationSource)
	if err != nil {
		t.Fatal(err)
	}
	inst := NewInstrument()
	if !inst.CouldHandle(&api.CompileOptions{Package: "runtime", Lang: "go1.26"}) {
		t.Fatal("the runtime instrument should handle the runtime package")
	}
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		inst.FilterAndEdit("proc.go", file, cursor, []*dst.File{file})
		return true
	}, nil)
	var buf bytes.Buffer
	if err := decorator.Fprint(&buf, file); err != nil {
		t.Fatal(err)
	}
	code := buf.String()

	// taken only on a user goroutine without runtime locks (the GC workers are
	// started between acquirem and releasem), right before the switch to the
	// system stack, after any allocation that could start GC workers
	guard := `gp == gp.m.curg && gp.m.locks == 0 && gp.m.preemptoff == ""`
	takeSnapshot := "gp.skywalking_tls_snapshot = goroutineChange(gp.skywalking_tls)"
	for _, name := range []string{"newproc", "newcoro"} {
		body := funcSource(code, name)
		snapshot, guarded, switched := strings.Index(body, takeSnapshot), strings.Index(body, guard), strings.Index(body, "systemstack(")
		if snapshot < 0 || guarded < 0 || guarded > snapshot || switched < snapshot {
			t.Errorf("%s should take the guarded context snapshot before switching to the system stack, got:\n%s", name, body)
		}
	}
	if body := funcSource(code, "newcoro"); strings.Index(body, "new(coro)") > strings.Index(body, takeSnapshot) {
		t.Errorf("newcoro should take the context snapshot after its allocations, got:\n%s", body)
	}
	newproc1 := funcSource(code, "newproc1")
	if strings.Contains(newproc1, "goroutineChange") {
		t.Errorf("newproc1 must not take the context snapshot on the system stack, got:\n%s", newproc1)
	}
	for _, stmt := range []string{".skywalking_tls = callergp.skywalking_tls_snapshot", "callergp.skywalking_tls_snapshot = nil"} {
		if !strings.Contains(newproc1, stmt) {
			t.Errorf("newproc1 should contain %q, got:\n%s", stmt, newproc1)
		}
	}
	if g, _, _ := strings.Cut(code, "func "); !strings.Contains(g, "skywalking_tls ") ||
		!strings.Contains(g, "skywalking_tls_snapshot ") {
		t.Errorf("g should have the context fields, got:\n%s", g)
	}
}

func funcSource(code, name string) string {
	start := strings.Index(code, "func "+name+"(")
	if start < 0 {
		return ""
	}
	end := strings.Index(code[start+1:], "\nfunc ")
	if end < 0 {
		return code[start:]
	}
	return code[start : start+1+end]
}
