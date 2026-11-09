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

package plugins

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"

	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

// renderDelegator renders the before and after functions that the agent
// generates for a method.
func renderDelegator(t *testing.T) (before, after string) {
	t.Helper()
	file, err := decorator.Parse(`package gin

type Engine struct{}

func (e *Engine) handle(path string) (int, error) {
	return 0, nil
}
`)
	if err != nil {
		t.Fatal(err)
	}
	var fn *dst.FuncDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*dst.FuncDecl); ok {
			fn = f
		}
	}
	enhance := NewMethodEnhance(NewTestInstrument("github.com/gin-gonic/gin"),
		&instrument.Point{Interceptor: "HandleInterceptor"}, fn, "/gin/engine.go", tools.CreateImportAnalyzer())
	for _, decl := range enhance.BuildForDelegator() {
		f, ok := decl.(*dst.FuncDecl)
		if !ok {
			continue
		}
		var buf bytes.Buffer
		if err := decorator.Fprint(&buf, &dst.File{Name: dst.NewIdent("gin"), Decls: []dst.Decl{f}}); err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "", buf.String(), 0); err != nil {
			t.Fatalf("the generated %s is not valid Go: %v\n%s", f.Name.Name, err, buf.String())
		}
		switch f.Name.Name {
		case enhance.AdapterPreFuncName:
			before = buf.String()
		case enhance.AdapterPostFuncName:
			after = buf.String()
		}
	}
	if before == "" || after == "" {
		t.Fatal("the before or after function was not generated")
	}
	return before, after
}

func indexOf(t *testing.T, src, part string) int {
	t.Helper()
	i := strings.Index(src, part)
	if i < 0 {
		t.Fatalf("%q is missing from the generated code:\n%s", part, src)
	}
	return i
}

// The before function marks the active span before BeforeInvoke, rolls it back
// when BeforeInvoke fails or panics, and finishes the invocation when the
// call is skipped, because the after function is not called then.
func TestBeforeTemplateRollsBackFailedHook(t *testing.T) {
	before, _ := renderDelegator(t)
	mark := indexOf(t, before, "operator.markActiveSpan()")
	invoke := indexOf(t, before, ".BeforeInvoke(invocation)")
	if mark > invoke {
		t.Fatal("the active span must be marked before BeforeInvoke")
	}
	if n := strings.Count(before, "operator.rollbackActiveSpan(invocation, _sw_span_mark, _sw_span_depth)"); n != 2 {
		t.Fatalf("the rollback should run on a panic and on an error, found %d calls:\n%s", n, before)
	}
	recoverAt := indexOf(t, before, "recover()")
	if recoverAt > mark {
		t.Fatal("the panic rollback must be deferred before the mark is taken")
	}
	indexOf(t, before, "var _sw_span_depth = -1")
	skip := indexOf(t, before, "if invocation.isContinue")
	exited := indexOf(t, before, "invocation.markExited()")
	if exited < skip {
		t.Fatal("the skipped call must finish the invocation")
	}
	// a failed conversion of the return values must not finish it, because
	// the call and the after-hook still run then
	if conversion := indexOf(t, before, "invocation.returnValues[0]"); exited < conversion {
		t.Fatal("the skipped call must be finished after the return values are converted")
	}
}

// The after function finishes the invocation last, even when AfterInvoke panics.
func TestAfterTemplateFinishesInvocation(t *testing.T) {
	_, after := renderDelegator(t)
	exited := indexOf(t, after, "defer invocation.markExited()")
	if recoverAt := indexOf(t, after, "recover()"); exited > recoverAt {
		t.Fatal("markExited must be deferred first, so it runs after the recover")
	}
	if invoke := indexOf(t, after, ".AfterInvoke(invocation"); exited > invoke {
		t.Fatal("markExited must be deferred before AfterInvoke runs")
	}
}
