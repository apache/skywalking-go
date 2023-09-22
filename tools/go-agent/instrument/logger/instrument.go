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

package logger

import (
	// for file import
	_ "embed"

	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/logger/frameworks"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var logFrameworks = []frameworks.LogFramework{
	frameworks.NewLogrus(),
	frameworks.NewZap(),
	// frameworks.NewBuildin(),
}

//go:embed context.go
var contextFile string

type Instrument struct {
	compileOpts       *api.CompileOptions
	framework         frameworks.LogFramework
	packageConf       *frameworks.PackageConfiguration
	automaticFunc     []*AutomaticFunctionInfo
	customizedReplace map[string]map[string]string
}

func NewInstrument() *Instrument {
	return &Instrument{
		customizedReplace: make(map[string]map[string]string),
	}
}

func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	for _, f := range logFrameworks {
		for p, conf := range f.PackagePaths() {
			if p == opts.Package {
				i.compileOpts = opts
				i.framework = f
				i.packageConf = conf
				return true
			}
		}
	}
	return false
}

func (i *Instrument) FilterAndEdit(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	// add to invoke check, if the method should add automatic bind
	if fun, ok := cursor.Node().(*dst.FuncDecl); ok {
		if body := i.framework.AutomaticBindFunctions(fun); body != "" {
			i.addAutomaticBindFunc(path, curFile, fun, body)
			return true
		}
	}
	replacements, needs := i.framework.CustomizedEnhance(path, curFile, cursor, allFiles)
	// adding replacements if needs
	if needs && len(replacements) > 0 {
		r := i.customizedReplace[path]
		if r == nil {
			r = make(map[string]string)
			i.customizedReplace[path] = r
		}
		for k, v := range replacements {
			r[k] = v
		}
	}
	return needs
}

func (i *Instrument) addAutomaticBindFunc(path string, curFile dst.Node, fun *dst.FuncDecl, body string) {
	automaticBind, err := frameworks.FrameworkFS.ReadFile("templates/automatic_bind.tmpl")
	if err != nil {
		panic(err)
	}
	funcID := tools.BuildFuncIdentity(i.compileOpts.Package, fun)
	var generateFuncName = fmt.Sprintf("%sautomaticLoggerBind%s", rewrite.GenerateMethodPrefix, funcID)
	var replaceName = fmt.Sprintf("//goagent:bind_%s\n", funcID)
	importAnalyzer := tools.CreateImportAnalyzer()
	importAnalyzer.AnalyzeFileImports(path, curFile)
	funcInvoker := tools.ExecuteTemplate(string(automaticBind), struct {
		AutomaticBindFuncName string
		Recvs                 []*tools.ParameterInfo
		Parameters            []*tools.ParameterInfo
		Results               []*tools.ParameterInfo
	}{
		AutomaticBindFuncName: generateFuncName,
		Recvs:                 tools.EnhanceParameterNames(fun.Recv, tools.FieldListTypeRecv),
		Parameters:            tools.EnhanceParameterNames(fun.Type.Params, tools.FieldListTypeParam),
		Results:               tools.EnhanceParameterNames(fun.Type.Results, tools.FieldListTypeResult),
	})
	importAnalyzer.AnalyzeNeedsImports(path, fun.Recv)
	importAnalyzer.AnalyzeNeedsImports(path, fun.Type.Params)
	importAnalyzer.AnalyzeNeedsImports(path, fun.Type.Results)
	i.automaticFunc = append(i.automaticFunc, &AutomaticFunctionInfo{
		Path:           path,
		Func:           fun,
		FuncName:       generateFuncName,
		RealInvoker:    funcInvoker,
		ReplaceName:    replaceName,
		DelegateBody:   body,
		ImportAnalyzer: importAnalyzer,
	})

	fun.Body.Decs.Lbrace.Prepend("\n", replaceName)
}

func (i *Instrument) AfterEnhanceFile(fromPath, newPath string) error {
	contentBytes, err := os.ReadFile(newPath)
	if err != nil {
		return err
	}

	// update the file content if needed
	content := string(contentBytes)
	var oldContent = content
	for _, enhance := range i.automaticFunc {
		if enhance.Path == fromPath {
			content = strings.Replace(content, enhance.ReplaceName, enhance.RealInvoker, 1)
		}
	}
	if replacements, existing := i.customizedReplace[fromPath]; existing {
		for k, v := range replacements {
			content = strings.Replace(content, k, v, 1)
		}
	}
	if oldContent == content {
		return nil
	}

	return os.WriteFile(newPath, []byte(content), 0o600)
}

func (i *Instrument) WriteExtraFiles(dir string) ([]string, error) {
	packageName := filepath.Base(i.compileOpts.Package)
	var contextRewriteFile *rewrite.FileInfo
	if i.compileOpts.DebugDir == "" {
		contextRewriteFile = rewrite.NewFile(packageName, "context.go", contextFile)
	} else {
		contextRewriteFile = rewrite.NewFileWithDebug(packageName, "context.go", contextFile,
			filepath.Join(i.compileOpts.DebugDir, "tools", "go-agent", "instrument", "logger"))
	}
	delegatorContent, err := i.writeDelegatorFile(packageName)
	if err != nil {
		return nil, err
	}
	delegatorFile := rewrite.NewFile(packageName, "delegator.go", delegatorContent)

	ctx := rewrite.NewContext(i.compileOpts.Package, packageName)
	files := []*rewrite.FileInfo{
		contextRewriteFile, delegatorFile,
	}
	generateExtraFiles, err := i.framework.GenerateExtraFiles(i.compileOpts.Package, i.compileOpts.DebugDir)
	if err != nil {
		return nil, err
	}
	files = append(files, generateExtraFiles...)
	if i.packageConf.NeedsHelpers {
		files = append(files, rewrite.NewFile(packageName, "skywalking_init.go",
			i.generateInitLoggerFileContent(packageName)))
	}
	extraFiles, err := ctx.MultipleFilesWithWritten("skywalking_", dir, packageName, files)
	if err != nil {
		return nil, err
	}

	return extraFiles, nil
}

func (i *Instrument) writeDelegatorFile(pkgName string) (string, error) {
	importDecl := &dst.GenDecl{
		Tok: token.IMPORT,
		Specs: []dst.Spec{
			&dst.ImportSpec{Name: dst.NewIdent("_"), Path: &dst.BasicLit{
				Kind: token.STRING, Value: fmt.Sprintf("%q", "unsafe"),
			}},
			&dst.ImportSpec{Path: &dst.BasicLit{
				Kind: token.STRING, Value: fmt.Sprintf("%q", i.compileOpts.Package),
			}},
		},
	}
	delegator := &dst.File{
		Name: dst.NewIdent(pkgName),
		Decls: []dst.Decl{
			importDecl,
		},
	}

	// add automatic function delegators
	i.addAutomaticFuncDelegators(delegator, importDecl)

	return tools.GenerateDSTFileContent(delegator, nil)
}

func (i *Instrument) generateInitLoggerFileContent(pkgName string) string {
	initTmpl, err := frameworks.FrameworkFS.ReadFile("templates/init.tmpl")
	if err != nil {
		panic(fmt.Sprintf("cannot found init template in logger framerwork: %v", err))
	}

	initFunctions := i.framework.InitFunctions()
	initFuncNames := make([]string, 0)
	for _, initFunc := range initFunctions {
		initFuncNames = append(initFuncNames, initFunc.Name.Name)
	}
	importsMap := make(map[string]string)
	for _, imp := range i.framework.InitImports() {
		name := filepath.Base(strings.TrimSuffix(imp.Path.Value, "\""))
		if imp.Name != nil && imp.Name.Name != "" {
			name = imp.Name.Name
		}
		importsMap[name] = imp.Path.Value
	}
	initDecls := tools.GoStringToDecls(tools.ExecuteTemplate(string(initTmpl), struct {
		Imports                     map[string]string
		NeedsVariables              bool
		NeedsChangeLoggerFunc       bool
		GetGlobalOperatorLinkMethod string
		SetGlobalLoggerLinkMethod   string
		OperatorTypeName            string
		LogTypeInConfig             *config.Log
		ConfigTypeAutomaticValue    string
		CurrentLogTypeName          string
		GetOperatorMethodName       string
		ChangeLoggerMethodName      string
		LogTracingEnableVarName     string
		LogTracingContextKeyVarName string
		LogReporterEnableVarName    string
		LogReporterLabelsVarName    string
		LogReportFuncName           string
		InitFunctionNames           []string
	}{
		Imports:                     importsMap,
		NeedsVariables:              i.packageConf.NeedsVariables,
		NeedsChangeLoggerFunc:       i.packageConf.NeedsChangeLoggerFunc,
		GetGlobalOperatorLinkMethod: consts.GlobalTracerGetMethodName,
		SetGlobalLoggerLinkMethod:   consts.GlobalLoggerSetMethodName,
		OperatorTypeName:            "Operator",
		LogTypeInConfig:             &config.GetConfig().Log,
		ConfigTypeAutomaticValue:    config.ConfigTypeAutomatic,
		CurrentLogTypeName:          i.framework.Name(),
		GetOperatorMethodName:       "GetOperator",
		ChangeLoggerMethodName:      "ChangeLogger",
		LogTracingEnableVarName:     "LogTracingContextEnable",
		LogTracingContextKeyVarName: "LogTracingContextKey",
		LogReporterEnableVarName:    "LogReporterEnable",
		LogReporterLabelsVarName:    "LogReporterLabelKeys",
		LogReportFuncName:           "ReportLog",
		InitFunctionNames:           initFuncNames,
	}))
	for _, f := range initFunctions {
		initDecls = append(initDecls, f)
	}

	f := &dst.File{Name: dst.NewIdent(pkgName), Decls: initDecls}
	if c, err1 := tools.GenerateDSTFileContent(f, nil); err1 != nil {
		panic(fmt.Errorf("generate logger init file error: %v", err))
	} else {
		return c
	}
}

func (i *Instrument) generatePackageNameWithTitle() string {
	return cases.Title(language.English).String(filepath.Base(i.compileOpts.Package))
}

func (i *Instrument) addAutomaticFuncDelegators(f *dst.File, importDecl *dst.GenDecl) {
	packageName := filepath.Base(i.compileOpts.Package)
	for _, fun := range i.automaticFunc {
		fun.ImportAnalyzer.AppendUsedImports(importDecl)
		delegatorFunc := &dst.FuncDecl{
			Name: dst.NewIdent(fun.FuncName),
			Type: &dst.FuncType{
				Params: &dst.FieldList{},
			},
		}

		for i, recv := range tools.EnhanceParameterNamesWithPackagePrefix(packageName, fun.Func.Recv, tools.FieldListTypeRecv) {
			delegatorFunc.Type.Params.List = append(delegatorFunc.Type.Params.List, &dst.Field{
				Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("recv_%d", i))},
				Type:  &dst.StarExpr{X: recv.PackagedType()},
			})
		}

		for i, parameter := range tools.EnhanceParameterNamesWithPackagePrefix(packageName, fun.Func.Type.Params, tools.FieldListTypeParam) {
			packagedType := parameter.PackagedType()
			// if the parameter is dynamic list, then change it to the array type
			if el, ok := packagedType.(*dst.Ellipsis); ok {
				packagedType = &dst.ArrayType{Elt: el.Elt}
			}
			delegatorFunc.Type.Params.List = append(delegatorFunc.Type.Params.List, &dst.Field{
				Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("param_%d", i))},
				Type:  &dst.StarExpr{X: packagedType},
			})
		}

		for i, result := range tools.EnhanceParameterNamesWithPackagePrefix(packageName, fun.Func.Type.Results, tools.FieldListTypeResult) {
			delegatorFunc.Type.Params.List = append(delegatorFunc.Type.Params.List, &dst.Field{
				Names: []*dst.Ident{dst.NewIdent(fmt.Sprintf("ret_%d", i))},
				Type:  &dst.StarExpr{X: result.PackagedType()},
			})
		}

		delegatorFunc.Body = &dst.BlockStmt{
			List: tools.GoStringToStats(fun.DelegateBody),
		}

		f.Decls = append(f.Decls, delegatorFunc)
	}
}

type AutomaticFunctionInfo struct {
	Path           string
	Func           *dst.FuncDecl
	FuncName       string
	RealInvoker    string
	ReplaceName    string
	DelegateBody   string
	ImportAnalyzer *tools.ImportAnalyzer
}
