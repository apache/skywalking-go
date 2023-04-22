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
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/agentcore"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var GenerateMethodPrefix = "_skywalking_enhance_"
var GenerateVarPrefix = "_skywalking_var_"

var methodEnhanceAdapterFiles = make(map[string]bool)

type MethodEnhance struct {
	funcDecl    *dst.FuncDecl
	path        string
	packageName string
	fullPackage string

	InstrumentName           string
	InterceptorDefineName    string
	InterceptorGeneratedName string
	InterceptorVarName       string

	Parameters []*tools.ParameterInfo
	Recvs      []*tools.ParameterInfo
	Results    []*tools.ParameterInfo

	FuncID              string
	AdapterPreFuncName  string
	AdapterPostFuncName string

	replacementKey   string
	replacementValue string
}

func NewMethodEnhance(inst instrument.Instrument, matcher *instrument.Point, f *dst.FuncDecl, path string) *MethodEnhance {
	fullPackage := filepath.Join(inst.BasePackage(), matcher.PackagePath)
	enhance := &MethodEnhance{
		funcDecl:              f,
		path:                  path,
		fullPackage:           fullPackage,
		packageName:           filepath.Base(fullPackage),
		InstrumentName:        inst.Name(),
		InterceptorDefineName: matcher.Interceptor,
		Parameters:            tools.EnhanceParameterNames(f.Type.Params, false),
		Results:               tools.EnhanceParameterNames(f.Type.Results, true),
	}
	if f.Recv != nil {
		enhance.Recvs = tools.EnhanceParameterNames(f.Recv, false)
	}

	enhance.FuncID = buildFrameworkFuncID(filepath.Join(inst.BasePackage(), matcher.PackagePath), f)
	enhance.AdapterPreFuncName = fmt.Sprintf("%s%s", rewrite.GenerateMethodPrefix, enhance.FuncID)
	enhance.AdapterPostFuncName = fmt.Sprintf("%s%s_ret", rewrite.GenerateMethodPrefix, enhance.FuncID)

	// the interceptor name needs to add the function id ensure there no conflict in the framework package
	titleCase := cases.Title(language.English)
	packageTitle := filepath.Base(titleCase.String(filepath.Join(inst.BasePackage(), matcher.PackagePath)))
	enhance.InterceptorGeneratedName = fmt.Sprintf("%s%s%s", rewrite.TypePrefix, packageTitle, enhance.InterceptorDefineName)
	enhance.InterceptorVarName = fmt.Sprintf("%sinterceptor_%s", rewrite.GenerateVarPrefix, enhance.FuncID)
	return enhance
}

func (m *MethodEnhance) BuildForInvoker() {
	insertsTmpl, err := templatesFS.ReadFile("templates/method_inserts.tmpl")
	if err != nil {
		panic(fmt.Errorf("reading method inserts: %w", err))
	}
	result := tools.ExecuteTemplate(string(insertsTmpl), m)
	m.replacementKey = fmt.Sprintf("//goagent:enhance_%s\n", m.FuncID)
	m.replacementValue = result

	m.funcDecl.Body.Decs.Lbrace.Prepend("\n", m.replacementKey)
}

func (m *MethodEnhance) BuildForAdapter() []dst.Decl {
	result := make([]dst.Decl, 0)
	if !methodEnhanceAdapterFiles[m.path] {
		// append the import for logger, one file only need import once
		result = append(result, tools.GoStringToDecls(fmt.Sprintf(`import (
	"%s/log"
	"%s/operator"

	%s "%s"	 // current enhancing package path, for rewrite phase in next step
)`, agentcore.EnhanceFromBasePackage, agentcore.EnhanceFromBasePackage, m.packageName, m.fullPackage))...)
		methodEnhanceAdapterFiles[m.path] = true
	}

	result = append(result, tools.GoStringToDecls(fmt.Sprintf(`var %s = &%s{}`, m.InterceptorVarName, m.InterceptorGeneratedName))...)
	preFunc := &dst.FuncDecl{
		Name: &dst.Ident{Name: m.AdapterPreFuncName},
		Type: &dst.FuncType{
			Params:  &dst.FieldList{},
			Results: &dst.FieldList{},
		},
	}
	for i, recv := range m.Recvs {
		preFunc.Type.Params.List = append(preFunc.Type.Params.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("recv_%d", i))},
			Type:  &dst.StarExpr{X: m.addPackagePrefixForArgsAndClone(m.packageName, recv.Type)},
		})
	}
	for i, parameter := range m.Parameters {
		preFunc.Type.Params.List = append(preFunc.Type.Params.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("param_%d", i))},
			Type:  &dst.StarExpr{X: m.addPackagePrefixForArgsAndClone(m.packageName, parameter.Type)},
		})
	}
	for i, result := range m.Results {
		preFunc.Type.Results.List = append(preFunc.Type.Results.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("ret_%d", i))},
			Type:  &dst.StarExpr{X: m.addPackagePrefixForArgsAndClone(m.packageName, result.Type)},
		})
	}
	preFunc.Type.Results.List = append(preFunc.Type.Results.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent("inv")},
		Type:  &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("operator"), Sel: dst.NewIdent("Invocation")}},
	}, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent("keep")},
		Type:  dst.NewIdent("bool"),
	})

	beforeFile, err := templatesFS.ReadFile("templates/method_intercept_before.tmpl")
	if err != nil {
		panic(fmt.Errorf("reading method before intercept template failure: %w", err))
	}
	preFunc.Body = &dst.BlockStmt{
		List: tools.GoStringToStats(tools.ExecuteTemplate(string(beforeFile), m)),
	}
	result = append(result, preFunc)

	postFunc := &dst.FuncDecl{
		Name: &dst.Ident{Name: m.AdapterPostFuncName},
		Type: &dst.FuncType{
			Params:  &dst.FieldList{},
			Results: &dst.FieldList{},
		},
	}
	postFunc.Type.Params.List = append(postFunc.Type.Params.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent("invocation")},
		Type:  &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("operator"), Sel: dst.NewIdent("Invocation")}},
	})
	for inx, f := range m.Results {
		postFunc.Type.Params.List = append(postFunc.Type.Params.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("ret_%d", inx))},
			Type:  &dst.StarExpr{X: m.addPackagePrefixForArgsAndClone(m.packageName, f.Type)},
		})
	}
	afterFile, err := templatesFS.ReadFile("templates/method_intercept_after.tmpl")
	if err != nil {
		panic(fmt.Errorf("reading method after intercept template failure: %w", err))
	}
	postFunc.Body = &dst.BlockStmt{
		List: tools.GoStringToStats(tools.ExecuteTemplate(string(afterFile), m)),
	}
	result = append(result, postFunc)
	return result
}

func (m *MethodEnhance) addPackagePrefixForArgsAndClone(pkg string, tp dst.Expr) dst.Expr {
	switch t := tp.(type) {
	case *dst.Ident:
		if rewrite.IsBasicDataType(t.Name) {
			return dst.Clone(tp).(dst.Expr)
		}
		// otherwise, add the package prefix
		return &dst.SelectorExpr{
			X:   dst.NewIdent(pkg),
			Sel: dst.NewIdent(t.Name),
		}
	case *dst.StarExpr:
		t.X = m.addPackagePrefixForArgsAndClone(pkg, t.X)
		return t
	default:
		return dst.Clone(tp).(dst.Expr)
	}
}

func (m *MethodEnhance) ReplaceFileContent(path, content string) string {
	if m.path == path {
		return strings.Replace(content, m.replacementKey, m.replacementValue, 1)
	}
	return content
}

func buildFrameworkFuncID(pkgPath string, node *dst.FuncDecl) string {
	var receiver string
	if node.Recv != nil {
		expr, ok := node.Recv.List[0].Type.(*dst.StarExpr)
		if !ok {
			return ""
		}
		ident, ok := expr.X.(*dst.Ident)
		if !ok {
			return ""
		}
		receiver = ident.Name
	}
	return fmt.Sprintf("%s_%s%s",
		regexp.MustCompile(`[/.\-@]`).ReplaceAllString(pkgPath, "_"), receiver, node.Name)
}
