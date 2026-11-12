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

package rewrite

import "testing"

// A builtin function called in a copied agent core file, such as close in the
// operator package, must keep its name.
func TestBuiltinFunctionsAreNotRenamed(t *testing.T) {
	c := &Context{}
	for _, name := range []string{"append", "cap", "clear", "close", "complex", "copy", "delete", "imag",
		"len", "make", "max", "min", "new", "panic", "print", "println", "real", "recover"} {
		if !c.callIsBasicNamesOrEnhanceName(name) || !c.typeIsBasicTypeValueOrEnhanceName(name) {
			t.Errorf("the builtin %s would be renamed", name)
		}
		info := &rewriteImportInfo{ctx: c, isAgentCore: true, pkgName: "operator"}
		if got := info.generateStaticMethod(name).Name; got != name {
			t.Errorf("the builtin %s is renamed to %s", name, got)
		}
	}
	if c.callIsBasicNamesOrEnhanceName("markExited") {
		t.Error("a function of the copied package must be renamed")
	}
}
