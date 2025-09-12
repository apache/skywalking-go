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

package tools

import (
	"fmt"
	"go/parser"
	"reflect"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

func TestChangePackageImportPath(t *testing.T) {
	tests := []struct {
		file       string
		pkgUpdates map[string]string
		actual     []string
	}{
		{
			file: `package main
import (
	"github.com/apache/skywalking-go/plugins/core/operator"
)`,
			pkgUpdates: map[string]string{
				"github.com/apache/skywalking-go/plugins/core/operator": "github.com/apache/skywalking-go/agent/core/operator",
			},
			actual: []string{
				"github.com/apache/skywalking-go/agent/core/operator",
			},
		},
		{
			file: `package main
import (
	"fmt"
	"github.com/apache/skywalking-go/agent/core/operator"
)`,
			pkgUpdates: map[string]string{
				"github.com/apache/skywalking-go/plugins/core/operator": "github.com/apache/skywalking-go/agent/core/operator",
			},
			actual: []string{
				"fmt",
				"github.com/apache/skywalking-go/agent/core/operator",
			},
		},
	}

	for _, test := range tests {
		f, err := decorator.ParseFile(nil, "main.go", test.file, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		ChangePackageImportPath(f, test.pkgUpdates)
		actual := make([]string, 0)
		for _, i := range f.Imports {
			actual = append(actual, strings.TrimSuffix(strings.TrimPrefix(i.Path.Value, "\""), "\""))
		}
		if !reflect.DeepEqual(actual, test.actual) {
			t.Fatalf("expect %v, actual %v", test.actual, actual)
		}
	}
}

func TestDeletePackageImports(t *testing.T) {
	tests := []struct {
		goCode   string
		validate func(result dst.Node) bool
		isValue  bool
	}{
		{
			goCode:  "test.Count(1)",
			isValue: true,
			validate: func(result dst.Node) bool {
				call := result.(*dst.CallExpr)
				return reflect.DeepEqual(call.Fun, dst.NewIdent("Count"))
			},
		},
		{
			goCode:  "[]test.Int{}",
			isValue: true,
			validate: func(result dst.Node) bool {
				call := result.(*dst.CompositeLit)
				return reflect.DeepEqual(call.Type, &dst.ArrayType{
					Elt: dst.NewIdent("Int"),
				})
			},
		},
		{
			goCode: `type Object struct {
	value test.Int
}`,
			isValue: false,
			validate: func(result dst.Node) bool {
				structType := result.(*dst.GenDecl).Specs[0].(*dst.TypeSpec).Type.(*dst.StructType)
				return reflect.DeepEqual(structType.Fields.List[0].Type, dst.NewIdent("Int"))
			},
		},
	}

	for i, test := range tests {
		content := "import test \"testpackage/test\"\n"
		if test.isValue {
			content += "var val = " + test.goCode
		} else {
			content += test.goCode
		}
		decls := GoStringToDecls(content)

		file := &dst.File{Name: dst.NewIdent("dst"), Decls: decls}
		DeletePackageImports(file, "testpackage/test")
		if len(file.Decls) != 1 {
			t.Errorf("failure to delete package, current decl count: %d", len(file.Decls))
		}
		var actualResult dst.Node
		if test.isValue {
			actualResult = file.Decls[0].(*dst.GenDecl).Specs[0].(*dst.ValueSpec).Values[0]
		} else {
			actualResult = file.Decls[0]
		}
		if !test.validate(actualResult) {
			t.Fatalf("validate %d error, real get result: %v", i, actualResult)
		}
	}
}

func TestGenerateDSTFileContent(t *testing.T) {
	tests := []struct {
		fileContent   string
		debugInfo     *DebugInfo
		resultContent string
	}{
		{
			fileContent: `package main

import (
	"fmt"
)

func main() {
}
`,
			debugInfo: &DebugInfo{
				FilePath:     "/test/main.go",
				Line:         10,
				CheckOldLine: true,
			},
			resultContent: `package main

import (
	"fmt"
)

//line /test/main.go:10
func main() {
}
`,
		},
	}

	for i, test := range tests {
		file, err := decorator.ParseFile(nil, "main.go", test.fileContent, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		content, err := GenerateDSTFileContent(file, test.debugInfo)
		if err != nil {
			t.Fatal(err)
		}
		if content != test.resultContent {
			t.Fatalf("case %d: expect %s, actual %s", i, test.resultContent, content)
		}
	}
}

func TestImportAnalyzer(t *testing.T) {
	tests := []struct {
		imports     []string
		fieldsCode  string
		usedImports map[string]string
	}{
		{
			imports:    []string{`test1 "test/test1"`},
			fieldsCode: `v1 test1.Int`,
			usedImports: map[string]string{
				"test1": "test/test1",
			},
		},
		{
			imports:     []string{`test1 "test/test1"`},
			fieldsCode:  `v1 int`,
			usedImports: map[string]string{},
		},
		{
			imports:    []string{`test1 "test/test1"`, `test2 "test/test2"`},
			fieldsCode: `v1 []test2.Int`,
			usedImports: map[string]string{
				"test2": "test/test2",
			},
		},
	}

	for i, test := range tests {
		content := ""
		for _, imp := range test.imports {
			content += "import " + imp + "\n"
		}
		content += fmt.Sprintf("func test(%s) {}", test.fieldsCode)

		f := &dst.File{
			Name:  dst.NewIdent("main.go"),
			Decls: GoStringToDecls(content),
		}
		analyzer := CreateImportAnalyzer()
		analyzer.AnalyzeFileImports("main.go", f)
		analyzer.AnalyzeNeedsImports("main.go", f.Decls[len(f.Decls)-1].(*dst.FuncDecl).Type.Params)

		if len(test.usedImports) != len(analyzer.usedImports) {
			t.Fatalf("case %d: expect %d used imports, actual %d", i, len(test.usedImports), len(analyzer.usedImports))
		}
		for name, path := range test.usedImports {
			spec := analyzer.usedImports[name]
			if spec == nil {
				t.Fatalf("case %d: expect use %s, actual nil", i, name)
			}
			if spec.Path.Value != fmt.Sprintf("%q", path) {
				t.Fatalf("case %d: expect use %s, actual %s", i, path, spec.Path.Value)
			}
		}
	}
}
