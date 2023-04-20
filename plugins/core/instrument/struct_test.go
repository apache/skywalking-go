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

//go:embed struct_test.go
var structFS embed.FS

// nolint
type TestStruct struct {
	field1 interface{}
	field2 int
	field3 *TestStruct
	field4 *embed.FS
}

func TestStructFilter(t *testing.T) {
	var tests = []struct {
		filter StructFilterOption
		found  bool
	}{
		{
			filter: WithFieldExists("field1"),
			found:  true,
		},
		{
			filter: WithFieldExists("field5"),
			found:  false,
		},
		{
			filter: WithFiledType("field1", "interface{}"),
			found:  true,
		},
		{
			filter: WithFiledType("field3", "*TestStruct"),
			found:  true,
		},
		{
			filter: WithFiledType("field4", "*embed.FS"),
			found:  true,
		},
		{
			filter: WithFiledType("field2", "string"),
			found:  false,
		},
	}

	file, err := structFS.ReadFile("struct_test.go")
	assert.Nil(t, err, "reading file error: %v", err)
	f, err := decorator.ParseFile(nil, "struct.go", file, parser.ParseComments)
	assert.Nil(t, err, "parse file error: %v", err)
	var typeSpec *dst.TypeSpec
	dstutil.Apply(f, func(cursor *dstutil.Cursor) bool {
		if s, ok := cursor.Node().(*dst.TypeSpec); ok {
			typeSpec = s
			return false
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
	assert.NotNil(t, typeSpec, "cannot found the structure")
	for i, tt := range tests {
		got := tt.filter(typeSpec, []*dst.File{f})
		assert.Equal(t, tt.found, got, "not correct with case: %d", i)
	}
}
