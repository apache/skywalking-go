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
	"path/filepath"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

const (
	zapPackageRootPath = "go.uber.org/zap"
	LoggerTypeName     = "*Logger"
)

var zapStaticFuncNames = []string{
	"New", "NewNop", "NewProduction", "NewDevelopment", "NewExample",
}
var zapLoggerMethodNames = []string{
	"Log", "Debug", "Info", "Warn", "Error", "Panic", "Fatal", "DPanic",
}

type Zap struct {
}

func NewZap() *Zap {
	return &Zap{}
}

func (z *Zap) Name() string {
	return "zap"
}

func (z *Zap) PackagePaths() map[string]*PackageConfiguration {
	return map[string]*PackageConfiguration{
		zapPackageRootPath: {NeedsHelpers: true},
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
	if pkgPath == zapPackageRootPath {
		file, err := FrameworkFS.ReadFile("zap_adapt.go")
		if err != nil {
			panic(fmt.Errorf("get zap file error: %v", err))
		}

		result := make([]*rewrite.FileInfo, 0)
		if debugDir == "" {
			result = append(result, rewrite.NewFile("zap", "zap_adapt.go", string(file)))
		} else {
			result = append(result, rewrite.NewFileWithDebug("zap", "zap_adapt.go", string(file),
				filepath.Join(debugDir, "tools", "go-agent", "instrument", "logger", "frameworks")))
		}

		return result, nil
	}
	return nil, nil
}

func (z *Zap) CustomizedEnhance(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) (map[string]string, bool) {
	fun, ok := cursor.Node().(*dst.FuncDecl)
	if !ok {
		return nil, false
	}

	// enhance the *Logger.Info, Warn, etc. method for add tracing context field
	if fun.Recv == nil || len(fun.Recv.List) != 1 || fun.Type.Params == nil || len(fun.Type.Params.List) <= 1 {
		return nil, false
	}

	if tools.GenerateTypeNameByExp(fun.Recv.List[0].Type) == LoggerTypeName {
		return z.enhanceLoggerTracingContext(fun)
	} else if tools.GenerateTypeNameByExp(fun.Recv.List[0].Type) == "*SugaredLogger" {
		return z.enhanceSugaredLoggerTracingContext(fun)
	}

	return nil, false
}

func (z *Zap) enhanceSugaredLoggerTracingContext(fun *dst.FuncDecl) (map[string]string, bool) {
	if fun.Name.Name != "log" {
		return nil, false
	}
	parameters := tools.EnhanceParameterNames(fun.Type.Params, false)
	var contextParameter *tools.ParameterInfo
	for _, p := range parameters {
		if p.Name == "context" && p.TypeName == "[]interface{}" {
			contextParameter = p
			break
		}
	}
	if contextParameter == nil {
		return nil, false
	}
	funcID := tools.BuildFuncIdentity(zapPackageRootPath, fun)
	replaceKey := fmt.Sprintf("//goagent:enhance_%s", funcID)
	replaceValue := fmt.Sprintf("%s = %s%s(%s)",
		contextParameter.Name, rewrite.StaticMethodPrefix, "ZapAddZapTracingInterfaceField", contextParameter.Name)
	fun.Body.Decs.Lbrace.Prepend("\n", replaceKey)

	return map[string]string{replaceKey: replaceValue}, true
}

func (z *Zap) enhanceLoggerTracingContext(fun *dst.FuncDecl) (map[string]string, bool) {
	foundMethod := false

	for _, name := range zapLoggerMethodNames {
		if fun.Name.Name == name {
			foundMethod = true
			break
		}
	}
	if !foundMethod {
		return nil, false
	}

	parameterNames := tools.EnhanceParameterNames(fun.Type.Params, false)
	var fieldParameter *tools.ParameterInfo
	for _, p := range parameterNames {
		if p.TypeName == "...Field" {
			fieldParameter = p
			break
		}
	}
	if fieldParameter == nil {
		return nil, false
	}
	funcID := tools.BuildFuncIdentity(zapPackageRootPath, fun)
	replaceKey := fmt.Sprintf("//goagent:enhance_%s", funcID)
	replaceValue := fmt.Sprintf("%s = %s%s(%s)",
		fieldParameter.Name, rewrite.StaticMethodPrefix, "ZapAddZapTracingField", fieldParameter.Name)
	fun.Body.Decs.Lbrace.Prepend("\n", replaceKey)

	return map[string]string{replaceKey: replaceValue}, true
}
