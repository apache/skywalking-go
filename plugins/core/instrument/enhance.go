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

import "github.com/dave/dst"

type EnhanceType int

var (
	EnhanceTypeMethod EnhanceType = 1
	EnhanceTypeStruct EnhanceType = 2
)

type MethodFilterOption func(decl *dst.FuncDecl, files []*dst.File) bool
type StructFilterOption func(structType *dst.TypeSpec, files []*dst.File) bool

type EnhanceMatcher struct {
	Type          EnhanceType
	Name          string
	Receiver      string
	MethodFilters []MethodFilterOption
	StructFilters []StructFilterOption
}

// NewStaticMethodEnhance creates a new EnhanceMatcher for static method.
func NewStaticMethodEnhance(name string, filters ...MethodFilterOption) *EnhanceMatcher {
	return &EnhanceMatcher{Type: EnhanceTypeMethod, Name: name, MethodFilters: filters}
}

// NewMethodEnhance creates a new EnhanceMatcher for method.
func NewMethodEnhance(receiver, name string, filters ...MethodFilterOption) *EnhanceMatcher {
	return &EnhanceMatcher{Type: EnhanceTypeMethod, Name: name, Receiver: receiver, MethodFilters: filters}
}

// NewStructEnhance creates a new EnhanceMatcher for struct.
func NewStructEnhance(name string, filters ...StructFilterOption) *EnhanceMatcher {
	return &EnhanceMatcher{Type: EnhanceTypeStruct, Name: name, StructFilters: filters}
}

func verifyTypeName(exp dst.Expr, val string) bool {
	data := generateTypeNameByExp(exp)
	return data == val
}

func generateTypeNameByExp(exp dst.Expr) string {
	var data string
	switch n := exp.(type) {
	case *dst.StarExpr:
		data = "*" + generateTypeNameByExp(n.X)
	case *dst.TypeAssertExpr:
		data = generateTypeNameByExp(n.X)
	case *dst.InterfaceType:
		data = "interface{}"
	case *dst.Ident:
		data = n.Name
	case *dst.SelectorExpr:
		data = generateTypeNameByExp(n.X) + "." + generateTypeNameByExp(n.Sel)
	default:
		return ""
	}
	return data
}
