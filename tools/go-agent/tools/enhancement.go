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

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

const interfaceName = "interface{}"
const OtherPackageRefPrefix = "swref_"
const parameterAppender = ", "

type ParameterInfo struct {
	Name     string
	Type     dst.Expr
	TypeName string
}

type PackagedParameterInfo struct {
	ParameterInfo
	PackageName string
}

type FieldListType int

const (
	FieldListTypeParam FieldListType = iota
	FieldListTypeResult
	FieldListTypeRecv
)

func (f FieldListType) String() string {
	switch f {
	case FieldListTypeRecv:
		return "recv"
	case FieldListTypeParam:
		return "param"
	case FieldListTypeResult:
		return "result"
	}
	return ""
}

// EnhanceParameterNames enhance the parameter names if they are missing
func EnhanceParameterNames(fields *dst.FieldList, fieldType FieldListType) []*ParameterInfo {
	if fields == nil {
		return nil
	}
	result := make([]*ParameterInfo, 0)
	for i, f := range fields.List {
		var defineName string
		switch fieldType {
		case FieldListTypeParam:
			defineName = fmt.Sprintf("skywalking_param_%d", i)
		case FieldListTypeResult:
			defineName = fmt.Sprintf("skywalking_result_%d", i)
		case FieldListTypeRecv:
			defineName = fmt.Sprintf("skywalking_recv_%d", i)
		}
		if len(f.Names) == 0 {
			f.Names = []*dst.Ident{{Name: defineName}}
			result = append(result, newParameterInfo(defineName, f.Type))
		} else {
			for _, n := range f.Names {
				if n.Name == "_" {
					*n = *dst.NewIdent(defineName)
					break
				}
			}

			for _, n := range f.Names {
				result = append(result, newParameterInfo(n.Name, f.Type))
			}
		}
	}
	return result
}

func EnhanceParameterNamesWithPackagePrefix(pkg string, fields *dst.FieldList, fieldListType FieldListType) []*PackagedParameterInfo {
	params := EnhanceParameterNames(fields, fieldListType)
	result := make([]*PackagedParameterInfo, 0)
	for _, p := range params {
		result = append(result, &PackagedParameterInfo{ParameterInfo: *p, PackageName: pkg})
	}
	return result
}

func GoStringToStats(goString string) []dst.Stmt {
	parsed, err := decorator.Parse(fmt.Sprintf(`
package main
func main() {
%s
}`, goString))
	if err != nil {
		panic(fmt.Sprintf("parsing go failure: %v\n%s", err, goString))
	}

	return parsed.Decls[0].(*dst.FuncDecl).Body.List
}

func GoStringToDecls(goString string) []dst.Decl {
	parsed, err := decorator.Parse(fmt.Sprintf(`
package main
%s`, goString))
	if err != nil {
		panic(fmt.Sprintf("parsing go failure: %v\n%s", err, goString))
	}

	return parsed.Decls
}

func InsertStmtsBeforeBody(body *dst.BlockStmt, tmpl string, data interface{}) {
	body.List = append(GoStringToStats(ExecuteTemplate(tmpl, data)), body.List...)
}

func newParameterInfo(name string, tp dst.Expr) *ParameterInfo {
	result := &ParameterInfo{
		Name:     name,
		Type:     tp,
		TypeName: GenerateTypeNameByExp(tp),
	}
	return result
}

func (p *PackagedParameterInfo) PackagedType() dst.Expr {
	return addPackagePrefixForArgsAndClone(p.PackageName, p.Type)
}

func (p *PackagedParameterInfo) PackagedTypeName() string {
	return GenerateTypeNameByExp(p.PackagedType())
}

// nolint
func GenerateTypeNameByExp(exp dst.Expr) string {
	var data string
	switch n := exp.(type) {
	case *dst.StarExpr:
		data = "*" + GenerateTypeNameByExp(n.X)
	case *dst.TypeAssertExpr:
		data = GenerateTypeNameByExp(n.X)
	case *dst.InterfaceType:
		data = interfaceName
	case *dst.Ident:
		data = n.Name
	case *dst.SelectorExpr:
		data = GenerateTypeNameByExp(n.X) + "." + GenerateTypeNameByExp(n.Sel)
	case *dst.Ellipsis:
		data = "[]" + GenerateTypeNameByExp(n.Elt)
	case *dst.ArrayType:
		data = "[]" + GenerateTypeNameByExp(n.Elt)
	case *dst.FuncType:
		data = "func("
		if n.Params != nil && len(n.Params.List) > 0 {
			for i, f := range n.Params.List {
				if i > 0 {
					data += parameterAppender
				}
				data += GenerateTypeNameByExp(f.Type)
			}
		}
		data += ")"
		if n.Results != nil && len(n.Results.List) > 0 {
			data += "("
			for i, f := range n.Results.List {
				if i > 0 {
					data += parameterAppender
				}
				data += GenerateTypeNameByExp(f.Type)
			}
			data += ")"
		}
	default:
		return ""
	}
	return data
}

func addPackagePrefixForArgsAndClone(pkg string, tp dst.Expr) dst.Expr {
	switch t := tp.(type) {
	case *dst.Ident:
		if IsBasicDataType(t.Name) {
			return dst.Clone(tp).(dst.Expr)
		}
		// otherwise, add the package prefix
		return &dst.SelectorExpr{
			X:   dst.NewIdent(pkg),
			Sel: dst.NewIdent(t.Name),
		}
	case *dst.StarExpr:
		expr := dst.Clone(tp).(*dst.StarExpr)
		expr.X = addPackagePrefixForArgsAndClone(pkg, t.X)
		return expr
	case *dst.Ellipsis:
		expr := dst.Clone(tp).(*dst.Ellipsis)
		expr.Elt = addPackagePrefixForArgsAndClone(pkg, t.Elt)
		return expr
	case *dst.SelectorExpr:
		exp := dst.Clone(tp).(*dst.SelectorExpr)
		// if also contains a package prefix, then it could be reffed a package with same name
		// Such as current package name is "grpc", and ref another package named "grpc"
		// Usually it's used on a wrapper plugin
		if sel, ok := t.X.(*dst.Ident); ok && sel.Name == pkg {
			exp.X = dst.NewIdent(OtherPackageRefPrefix + pkg)
		}
		return exp
	default:
		return dst.Clone(tp).(dst.Expr)
	}
}
