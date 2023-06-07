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
	"strings"

	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/agentcore"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var methodEnhanceAdapterFiles = make(map[string]bool)
var methodImportAgentCorePackages = []string{
	"log", "tracing", "operator",
}

type MethodEnhance struct {
	funcDecl    *dst.FuncDecl
	path        string
	packageName string
	fullPackage string

	InstrumentName           string
	InterceptorDefineName    string
	InterceptorGeneratedName string
	InterceptorVarName       string

	Parameters []*tools.PackagedParameterInfo
	Recvs      []*tools.PackagedParameterInfo
	Results    []*tools.PackagedParameterInfo

	FuncID              string
	AdapterPreFuncName  string
	AdapterPostFuncName string

	replacementKey   string
	replacementValue string

	importAnalyzer *tools.ImportAnalyzer
}

func NewMethodEnhance(inst instrument.Instrument, matcher *instrument.Point, f *dst.FuncDecl, path string,
	importAnalyzer *tools.ImportAnalyzer) *MethodEnhance {
	fullPackage := filepath.Join(inst.BasePackage(), matcher.PackagePath)
	pkgName := filepath.Base(fullPackage)
	if matcher.PackageName != "" {
		pkgName = matcher.PackageName
	}
	enhance := &MethodEnhance{
		funcDecl:              f,
		path:                  path,
		fullPackage:           fullPackage,
		packageName:           pkgName,
		InstrumentName:        inst.Name(),
		InterceptorDefineName: matcher.Interceptor,
		Parameters:            tools.EnhanceParameterNamesWithPackagePrefix(pkgName, f.Type.Params, false),
		Results:               tools.EnhanceParameterNamesWithPackagePrefix(pkgName, f.Type.Results, true),
	}
	if f.Recv != nil {
		enhance.Recvs = tools.EnhanceParameterNamesWithPackagePrefix(pkgName, f.Recv, false)
	}

	importAnalyzer.AnalyzeNeedsImports(path, f.Type.Params)
	importAnalyzer.AnalyzeNeedsImports(path, f.Type.Results)
	enhance.importAnalyzer = importAnalyzer

	enhance.FuncID = tools.BuildFuncIdentity(filepath.Join(inst.BasePackage(), matcher.PackagePath), f)
	enhance.AdapterPreFuncName = fmt.Sprintf("%s%s", rewrite.GenerateMethodPrefix, enhance.FuncID)
	enhance.AdapterPostFuncName = fmt.Sprintf("%s%s_ret", rewrite.GenerateMethodPrefix, enhance.FuncID)

	// the interceptor name needs to add the function id ensure there no conflict in the framework package
	titleCase := cases.Title(language.English)
	packageTitle := filepath.Base(titleCase.String(filepath.Join(inst.BasePackage(), pkgName)))
	enhance.InterceptorGeneratedName = fmt.Sprintf("%s%s%s", rewrite.TypePrefix, packageTitle, enhance.InterceptorDefineName)
	enhance.InterceptorVarName = fmt.Sprintf("%sinterceptor_%s", rewrite.GenerateVarPrefix, enhance.FuncID)
	return enhance
}

func (m *MethodEnhance) PackageName() string {
	return m.packageName
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

func (m *MethodEnhance) BuildImports(decl *dst.GenDecl) {
	if !methodEnhanceAdapterFiles[filepath.Dir(m.path)] {
		for _, n := range methodImportAgentCorePackages {
			m.appendImport(decl, "", fmt.Sprintf("%s/%s", agentcore.EnhanceFromBasePackage, n))
		}
		m.appendImport(decl, m.packageName, m.fullPackage)
		methodEnhanceAdapterFiles[filepath.Dir(m.path)] = true
	}

	m.importAnalyzer.AppendUsedImports(decl)
}

func (m *MethodEnhance) appendImport(decl *dst.GenDecl, name, path string) {
	imp := &dst.ImportSpec{
		Path: &dst.BasicLit{
			Value: fmt.Sprintf("%q", path),
		},
	}
	if name != "" {
		imp.Name = dst.NewIdent(name)
	}
	decl.Specs = append(decl.Specs, imp)
}

func (m *MethodEnhance) BuildForDelegator() []dst.Decl {
	result := make([]dst.Decl, 0)

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
			Type:  &dst.StarExpr{X: recv.PackagedType()},
		})
	}
	for i, parameter := range m.Parameters {
		preFunc.Type.Params.List = append(preFunc.Type.Params.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("param_%d", i))},
			Type:  &dst.StarExpr{X: m.changeTypeIfNeeds(parameter.PackagedType())},
		})
	}
	for i, result := range m.Results {
		preFunc.Type.Results.List = append(preFunc.Type.Results.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("ret_%d", i))},
			Type:  result.PackagedType(),
		})
	}
	preFunc.Type.Results.List = append(preFunc.Type.Results.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent("invocation")},
		Type:  &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("operator"), Sel: dst.NewIdent("realInvocation")}},
	}, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent("skip")},
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
		Type:  &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("operator"), Sel: dst.NewIdent("realInvocation")}},
	})
	for inx, f := range m.Results {
		postFunc.Type.Params.List = append(postFunc.Type.Params.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("ret_%d", inx))},
			Type:  &dst.StarExpr{X: f.PackagedType()},
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

func (m *MethodEnhance) changeTypeIfNeeds(tp dst.Expr) dst.Expr {
	// change "...XXX" to "[]XXX" for reference type
	if el, ok := tp.(*dst.Ellipsis); ok {
		return &dst.ArrayType{Elt: el.Elt}
	}
	return tp
}

func (m *MethodEnhance) ReplaceFileContent(path, content string) string {
	if m.path == path {
		return strings.Replace(content, m.replacementKey, m.replacementValue, 1)
	}
	return content
}
