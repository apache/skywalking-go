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

package instrument

import (
	"embed"
	"go/parser"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"

	"github.com/stretchr/testify/assert"
)

//go:embed method_test.go
var methodFS embed.FS

// nolint
func testStaticMethod(i int) *TestStruct {
	return nil
}

// nolint
func (t *TestStruct) testMethod(d interface{}) {

}

func TestStaticMethodFilter(t *testing.T) {
	var tests = []struct {
		filter MethodFilterOption
		found  bool
	}{
		{
			filter: WithArgsCount(1),
			found:  true,
		},
		{
			filter: WithResultCount(1),
			found:  true,
		},
		{
			filter: WithArgType(0, "int"),
			found:  true,
		},
		{
			filter: WithStaticMethod(),
			found:  true,
		},
	}

	file, err := methodFS.ReadFile("method_test.go")
	assert.Nil(t, err, "reading file error: %v", err)
	f, err := decorator.ParseFile(nil, "method.go", file, parser.ParseComments)
	assert.Nil(t, err, "parse file error: %v", err)
	var funcDecl *dst.FuncDecl
	dstutil.Apply(f, func(cursor *dstutil.Cursor) bool {
		if s, ok := cursor.Node().(*dst.FuncDecl); ok && s.Name.Name == "testStaticMethod" {
			funcDecl = s
			return false
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
	assert.NotNil(t, funcDecl, "cannot found the function")
	for i, tt := range tests {
		got := tt.filter(funcDecl, []*dst.File{f})
		assert.Equal(t, tt.found, got, "not correct with case: %d", i)
	}
}

func TestReceiverMethodFilter(t *testing.T) {
	var tests = []struct {
		filter MethodFilterOption
		found  bool
	}{
		{
			filter: WithArgType(0, "interface{}"),
			found:  true,
		},
		{
			filter: WithResultCount(0),
			found:  true,
		},
		{
			filter: WithReceiverType("*TestStruct"),
			found:  true,
		},
	}

	file, err := methodFS.ReadFile("method_test.go")
	assert.Nil(t, err, "reading file error: %v", err)
	f, err := decorator.ParseFile(nil, "method.go", file, parser.ParseComments)
	assert.Nil(t, err, "parse file error: %v", err)
	var funcDecl *dst.FuncDecl
	dstutil.Apply(f, func(cursor *dstutil.Cursor) bool {
		if s, ok := cursor.Node().(*dst.FuncDecl); ok && s.Name.Name == "testMethod" {
			funcDecl = s
			return false
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
	assert.NotNil(t, funcDecl, "cannot found the function")
	for i, tt := range tests {
		got := tt.filter(funcDecl, []*dst.File{f})
		assert.Equal(t, tt.found, got, "not correct with case: %d", i)
	}
}
