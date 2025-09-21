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

package goroutine

import (
	"os"
	"path/filepath"
	runtimepkg "runtime"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

const mainPackage = "main"

type Instrument struct {
	opts           *api.CompileOptions
	helperInjected bool
}

func NewInstrument() *Instrument { return &Instrument{} }

// CouldHandle checks whether the given package should be instrumented.
// Returns false if:
//   - The package is not "main" and does not contain a dot (standard Go package).
//   - The package is part of the SkyWalking-Go project itself.
func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	i.opts = opts
	if opts.Package != mainPackage && !strings.Contains(opts.Package, ".") {
		return false
	}
	if strings.HasPrefix(opts.Package, "github.com/apache/skywalking-go/") {
		return false
	}
	return true
}

func (i *Instrument) FilterAndEdit(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	// skip stdlib/module cache
	if i.opts != nil {
		out := i.opts.Output
		if strings.Contains(out, "/usr/local/go/") ||
			strings.Contains(out, "/pkg/mod/") || strings.Contains(out, "\\pkg\\mod\\") {
			return false
		}
	}
	p := path
	goroot := runtimepkg.GOROOT()
	if (goroot != "" && strings.HasPrefix(p, filepath.Join(goroot, "src")+string(os.PathSeparator))) ||
		strings.HasPrefix(p, "/usr/local/go/src/") ||
		strings.Contains(p, "/pkg/mod/") || strings.Contains(p, "\\pkg\\mod\\") {
		return false
	}

	// Only process 'go' statements (goroutines)
	n, ok := cursor.Node().(*dst.GoStmt)
	if !ok || n.Call == nil {
		return false
	}

	// Mark that a helper declaration file needs to be generated
	if !i.helperInjected {
		i.helperInjected = true
	}

	ensureCall := &dst.ExprStmt{X: &dst.CallExpr{Fun: dst.NewIdent("skywalkingEnsureGoroutineLabels")}}

	switch call := n.Call.Fun.(type) {
	case *dst.FuncLit:
		body := call.Body
		body.List = append([]dst.Stmt{ensureCall}, body.List...)
	default:
		newLit := &dst.FuncLit{
			Type: &dst.FuncType{Params: &dst.FieldList{}},
			Body: &dst.BlockStmt{
				List: []dst.Stmt{
					ensureCall,
					&dst.ExprStmt{X: &dst.CallExpr{Fun: n.Call.Fun, Args: n.Call.Args}},
				},
			},
		}
		n.Call.Fun = newLit
		n.Call.Args = nil
	}

	return true
}

func (i *Instrument) AfterEnhanceFile(fromPath, newPath string) error { return nil }

func (i *Instrument) WriteExtraFiles(dir string) ([]string, error) {
	if !i.helperInjected || i.opts == nil {
		return nil, nil
	}
	out := i.opts.Output
	if strings.Contains(out, "/usr/local/go/") || strings.Contains(out, "/pkg/mod/") || strings.Contains(out, "\\pkg\\mod\\") {
		return nil, nil
	}
	pkg := i.opts.Package
	if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
		pkg = pkg[idx+1:]
	}
	if pkg == "" {
		pkg = mainPackage
	}
	// provided by runtime
	content := "package " + pkg + `

import _ "unsafe"

//go:linkname skywalkingEnsureGoroutineLabels skywalkingEnsureGoroutineLabels
var skywalkingEnsureGoroutineLabels func()
`
	return tools.WriteMultipleFile(dir, map[string]string{"skywalking_goroutine_helper.go": content})
}
