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
	"testing"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
)

func TestContainsDirective(t *testing.T) {
	var tests = []struct {
		goCode    string
		directive string
		contains  bool
	}{
		{
			goCode: `//skywalking:nocopy
func test() {}`,
			directive: consts.DirecitveNoCopy,
			contains:  true,
		},
		{
			goCode: `// test method
//skywalking:nocopy
// test method
func test1() {}`,
			directive: consts.DirecitveNoCopy,
			contains:  true,
		},
		{
			goCode: `func test1() {}
//skywalking:nocopy
`,
			directive: consts.DirecitveNoCopy,
			contains:  false,
		},
		{
			goCode: `// skywalking:nocopy
func test2() {}`,
			directive: consts.DirecitveNoCopy,
			contains:  false,
		},
	}

	for _, test := range tests {
		decls := GoStringToDecls(test.goCode)
		contains := ContainsDirective(decls[0], test.directive)
		if contains != test.contains {
			t.Errorf("ContainsDirective(%s, %s) = %v, excepted %v", test.goCode, test.directive, contains, test.contains)
		}
	}
}

func TestFindDirective(t *testing.T) {
	var tests = []struct {
		goCode    string
		directive string
		found     string
	}{
		{
			goCode: `//skywalking:nocopy
func test() {}`,
			directive: consts.DirecitveNoCopy,
			found:     "//skywalking:nocopy",
		},
		{
			goCode: `// test method
//skywalking:nocopy
// test method
func test1() {}`,
			directive: consts.DirecitveNoCopy,
			found:     "//skywalking:nocopy",
		},
		{
			goCode: `func test1() {}
//skywalking:nocopy
`,
			directive: consts.DirecitveNoCopy,
			found:     "",
		},
		{
			goCode: `//skywalking:native test method
func test2() {}`,
			directive: consts.DirectiveNative,
			found:     "//skywalking:native test method",
		},
	}

	for _, test := range tests {
		decls := GoStringToDecls(test.goCode)
		found := FindDirective(decls[0], test.directive)
		if found != test.found {
			t.Errorf("FindDirective(%s, %s) = %v, excepted %v", test.goCode, test.directive, found, test.found)
		}
	}
}
