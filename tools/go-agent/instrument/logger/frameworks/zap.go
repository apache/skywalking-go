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

package frameworks

import (
	"fmt"
	"go/token"
	"path/filepath"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

const (
	zapPackageRootPath = "go.uber.org/zap"
	zapPackageCorePath = "go.uber.org/zap/zapcore"
	LoggerTypeName     = "*Logger"
)

var zapStaticFuncNames = []string{
	"New", "NewNop", "NewProduction", "NewDevelopment", "NewExample",
}

type Zap struct {
	initFunction *dst.FuncDecl
	initImports  []*dst.ImportSpec
}

func NewZap() *Zap {
	return &Zap{}
}

func (z *Zap) Name() string {
	return "zap"
}

func (z *Zap) PackagePaths() map[string]*PackageConfiguration {
	return map[string]*PackageConfiguration{
		zapPackageRootPath: {NeedsHelpers: true, NeedsVariables: true, NeedsChangeLoggerFunc: true},
		zapPackageCorePath: {NeedsHelpers: true, NeedsVariables: false, NeedsChangeLoggerFunc: false},
	}
}

func (z *Zap) AutomaticBindFunctions(fun *dst.FuncDecl) string {
	if fun.Recv != nil || fun.Type.Results == nil || len(fun.Type.Results.List) < 1 ||
		tools.GenerateTypeNameByExp(fun.Type.Results.List[0].Type) != LoggerTypeName {
		return ""
	}
	foundName := false
	for _, n := range zapStaticFuncNames {
		if fun.Name.Name == n {
			foundName = true
			break
		}
	}
	if !foundName {
		return ""
	}

	return rewrite.StaticMethodPrefix + "ZapUpdateZapLogger(*ret_0)"
}

func (z *Zap) GenerateExtraFiles(pkgPath, debugDir string) ([]*rewrite.FileInfo, error) {
	var path string
	if pkgPath == zapPackageRootPath {
		path = "zap_root.go"
	} else if pkgPath == zapPackageCorePath {
		path = "zap_core.go"
	}

	file, err := FrameworkFS.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("get zap file error: %v", err))
	}

	result := make([]*rewrite.FileInfo, 0)
	if debugDir == "" {
		result = append(result, rewrite.NewFile("zap", path, string(file)))
	} else {
		result = append(result, rewrite.NewFileWithDebug("zap", path, string(file),
			filepath.Join(debugDir, "tools", "go-agent", "instrument", "logger", "frameworks")))
	}
	return result, nil
}

//nolint
func (z *Zap) CustomizedEnhance(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) (map[string]string, bool) {
	switch n := cursor.Node().(type) {
	case *dst.TypeSpec:
		// adding the context field into entry
		st, ok := n.Type.(*dst.StructType)
		if !ok || st.Fields == nil || n.Name == nil {
			return nil, false
		}
		if n.Name.Name == "CheckedEntry" {
			st.Fields.List = append(st.Fields.List,
				// tracing context object
				&dst.Field{Names: []*dst.Ident{dst.NewIdent("SWContext")}, Type: dst.NewIdent("interface{}")},
				// tracing context field
				&dst.Field{Names: []*dst.Ident{dst.NewIdent("SWContextField")}, Type: &dst.StarExpr{X: dst.NewIdent("Field")}},
				// existing fields needs to be added into, such as generate from log.With("key", "value")
				&dst.Field{Names: []*dst.Ident{dst.NewIdent("SWFields")}, Type: &dst.ArrayType{Elt: dst.NewIdent("Field")}},
			)
			curFile.Decls = append(curFile.Decls, &dst.GenDecl{
				Tok: token.VAR,
				Specs: []dst.Spec{
					&dst.ValueSpec{Names: []*dst.Ident{
						dst.NewIdent("SWReporterEnable"),
						dst.NewIdent("SWLogEnable"),
					}, Type: dst.NewIdent("bool")},
					&dst.ValueSpec{Names: []*dst.Ident{
						dst.NewIdent("SWReporterLabelKeys"),
					}, Type: dst.NewIdent("[]string")},
					&dst.ValueSpec{Names: []*dst.Ident{
						dst.NewIdent("SWLogTracingContextKey"),
					}, Type: dst.NewIdent("string")},
					&dst.ValueSpec{Names: []*dst.Ident{
						dst.NewIdent("SWFields"),
					}, Type: dst.NewIdent("[]Field")},
				},
			})
			return nil, true
		}
		if n.Name.Name == "Logger" {
			st.Fields.List = append(st.Fields.List,
				&dst.Field{Names: []*dst.Ident{dst.NewIdent("SWFields")}, Type: dst.NewIdent("[]zapcore.Field")})
			return nil, true
		}
	case *dst.FuncDecl:
		// enhance the method which check the log and generate the entry
		if n.Recv != nil && len(n.Recv.List) == 1 && tools.GenerateTypeNameByExp(n.Recv.List[0].Type) == "*Logger" &&
			n.Name.Name == "check" &&
			n.Type.Results != nil && len(n.Type.Results.List) > 0 &&
			tools.GenerateTypeNameByExp(n.Type.Results.List[0].Type) == "*zapcore.CheckedEntry" {
			entryName := tools.EnhanceParameterNames(n.Type.Results, true)[0].Name
			recvName := tools.EnhanceParameterNames(n.Recv, true)[0].Name

			// init the zapcore variables
			z.initFunction = tools.GoStringToDecls(fmt.Sprintf(`func initZapCore() {
zapcore.SWReporterEnable = %s
zapcore.SWReporterLabelKeys = %s
zapcore.SWLogEnable = %s
}`, "LogReporterEnable", "LogReporterLabelKeys", "LogTracingContextEnable"))[0].(*dst.FuncDecl)
			z.initImports = []*dst.ImportSpec{
				{Path: &dst.BasicLit{Kind: token.STRING, Value: `"go.uber.org/zap/zapcore"`}},
			}

			return z.enhanceMethod(n, fmt.Sprintf("defer func() {if %s != nil {"+
				"%s.SWContext, %s.SWContextField = %s%s(%s); %s.SWFields = %s.SWFields;}}()", entryName,
				entryName, entryName, rewrite.StaticMethodPrefix, "ZapTracingContextEnhance", entryName, entryName, recvName)), true
		}

		if n.Recv != nil && len(n.Recv.List) == 1 && n.Name.Name == "With" &&
			n.Type.Params != nil && len(n.Type.Params.List) == 1 &&
			tools.GenerateTypeNameByExp(n.Recv.List[0].Type) == "*Logger" && tools.GenerateTypeNameByExp(n.Type.Params.List[0].Type) == "[]Field" {
			recvs := tools.EnhanceParameterNames(n.Recv, false)
			parameters := tools.EnhanceParameterNames(n.Type.Params, false)
			results := tools.EnhanceParameterNames(n.Type.Results, true)

			return z.enhanceMethod(n, fmt.Sprintf(`defer func() {if %s != nil { %s.SWFields = %sZap%s(%s, %s.SWFields) }}()`,
				results[0].Name, results[0].Name, rewrite.StaticMethodPrefix, "KnownFieldFilter", parameters[0].Name, recvs[0].Name)), true
		}

		// enhance the method which write the checked entry context
		if n.Recv != nil && len(n.Recv.List) == 1 && tools.GenerateTypeNameByExp(n.Recv.List[0].Type) == "*CheckedEntry" &&
			n.Name.Name == "Write" &&
			n.Type.Params != nil && len(n.Type.Params.List) == 1 &&
			tools.GenerateTypeNameByExp(n.Type.Params.List[0].Type) == "[]Field" {
			recvs := tools.EnhanceParameterNames(n.Recv, false)
			parameters := tools.EnhanceParameterNames(n.Type.Params, false)
			return z.enhanceMethod(n, fmt.Sprintf(`if %s != nil { %s = %sZapcore%s(%s, %s, %s.SWFields, %s.SWContext, %s.SWContextField, SWReporterEnable, SWLogEnable, SWReporterLabelKeys) }`,
				recvs[0].Name, parameters[0].Name, rewrite.StaticMethodPrefix, "ReportLogFromZapEntry", recvs[0].Name,
				parameters[0].Name, recvs[0].Name, recvs[0].Name, recvs[0].Name)), true
		}
	}
	return nil, false
}

func (z *Zap) enhanceMethod(fun *dst.FuncDecl, goCode string) map[string]string {
	funcID := tools.BuildFuncIdentity(zapPackageRootPath, fun)
	replaceKey := fmt.Sprintf("//goagent:enhance_%s", funcID)
	fun.Body.Decs.Lbrace.Prepend("\n", replaceKey)

	return map[string]string{replaceKey: goCode}
}

func (z *Zap) InitFunctions() []*dst.FuncDecl {
	if z.initFunction != nil {
		return []*dst.FuncDecl{z.initFunction}
	}
	return nil
}

func (z *Zap) InitImports() []*dst.ImportSpec {
	return z.initImports
}
