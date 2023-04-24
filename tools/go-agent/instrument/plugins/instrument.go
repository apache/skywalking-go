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
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"

	"github.com/sirupsen/logrus"
)

//go:embed templates
var templatesFS embed.FS

type Instrument struct {
	realInst      instrument.Instrument
	methodFilters []*instrument.Point
	structFilters []*instrument.Point

	compileOpts *api.CompileOptions

	enhancements    []Enhance
	extraFilesWrote bool
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

type Enhance interface {
	BuildForDelegator() []dst.Decl
	ReplaceFileContent(path, content string) string
}

func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	for _, ins := range instruments {
		// must have the same base package prefix
		if !strings.HasPrefix(opts.Package, ins.BasePackage()) {
			continue
		}
		// check the version of the framework could handler
		version, err := i.tryToFindThePluginVersion(opts, ins)
		if err != nil {
			logrus.Warnf("ignore the plugin %s, because: %s", ins.Name(), err)
			continue
		}

		if ins.VersionChecker(version) {
			i.realInst = ins
			i.compileOpts = opts
			for _, p := range ins.Points() {
				switch p.At.Type {
				case instrument.EnhanceTypeMethod:
					i.methodFilters = append(i.methodFilters, p)
				case instrument.EnhanceTypeStruct:
					i.structFilters = append(i.structFilters, p)
				}
			}
			return true
		}
	}
	return false
}

func (i *Instrument) FilterAndEdit(path string, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	switch n := cursor.Node().(type) {
	case *dst.TypeSpec:
		for _, filter := range i.structFilters {
			if i.verifyPackageIsMatch(path, filter) && i.validateStructIsMatch(filter.At, n, allFiles) {
				i.enhanceStruct(i.realInst, filter, n, path)
				return true
			}
		}
	case *dst.FuncDecl:
		for _, filter := range i.methodFilters {
			if i.verifyPackageIsMatch(path, filter) && i.validateMethodInsMatch(filter.At, n, allFiles) {
				i.enhanceMethod(i.realInst, filter, n, path)
				return true
			}
		}
	}
	return false
}

func (i *Instrument) enhanceStruct(_ instrument.Instrument, _ *instrument.Point, typeSpec *dst.TypeSpec, _ string) {
	enhance := NewInstanceEnhance(typeSpec)
	enhance.EnhanceField()
	i.enhancements = append(i.enhancements, enhance)
}

func (i *Instrument) enhanceMethod(inst instrument.Instrument, matcher *instrument.Point, funcDecl *dst.FuncDecl, path string) {
	enhance := NewMethodEnhance(inst, matcher, funcDecl, path)
	enhance.BuildForInvoker()
	i.enhancements = append(i.enhancements, enhance)
}

func (i *Instrument) verifyPackageIsMatch(_ string, point *instrument.Point) bool {
	pointPackagePath := filepath.Join(i.realInst.BasePackage(), point.PackagePath)
	// check the package path
	return i.compileOpts.Package == pointPackagePath
}

func (i *Instrument) AfterEnhanceFile(fromPath, newPath string) error {
	contentBytes, err := os.ReadFile(newPath)
	if err != nil {
		return err
	}

	// update the file content if needed
	content := string(contentBytes)
	var oldContent = content
	for _, enhance := range i.enhancements {
		content = enhance.ReplaceFileContent(fromPath, content)
	}
	if oldContent == content {
		return nil
	}

	return os.WriteFile(newPath, []byte(content), 0o600)
}

func (i *Instrument) WriteExtraFiles(basePath string) ([]string, error) {
	// if no enhancements or already wrote extra files, then ignore
	if len(i.enhancements) == 0 || i.extraFilesWrote {
		return nil, nil
	}
	i.extraFilesWrote = true

	packageName := filepath.Base(i.compileOpts.Package)
	context := rewrite.NewContext(i.compileOpts.Package, packageName)

	var results = make([]string, 0)
	// write delegator file
	var files []string
	var err error
	if files, err = i.writeDelegatorFile(context, basePath); err != nil {
		return nil, err
	}
	results = append(results, files...)

	// copy basic support files(operators)
	if files, err = i.copyOperatorsFS(context, basePath, packageName); err != nil {
		return nil, err
	}
	results = append(results, files...)

	// copy user defined files(interceptors)
	if files, err = i.copyFrameworkFS(context, i.compileOpts.Package, basePath, packageName); err != nil {
		return nil, err
	}
	results = append(results, files...)
	return results, nil
}

func (i *Instrument) copyFrameworkFS(context *rewrite.Context, compilePkgFullPath, baseDir, packageName string) ([]string, error) {
	subPkgPath := strings.TrimPrefix(compilePkgFullPath, i.realInst.BasePackage())
	if subPkgPath == "" {
		subPkgPath = "."
	}

	var debugBaseDir string
	if i.compileOpts.DebugDir != "" {
		debugBaseDir = filepath.Join(i.compileOpts.DebugDir, "plugins", i.realInst.Name(), subPkgPath)
	}
	pkgCopiedEntries, err := i.realInst.FS().ReadDir(subPkgPath)
	if err != nil {
		return nil, err
	}
	files := make([]*rewrite.FileInfo, 0)
	for _, entry := range pkgCopiedEntries {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == "go.mod" || entry.Name() == "go.sum" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		readFile, err1 := fs.ReadFile(i.realInst.FS(), filepath.Join(subPkgPath, entry.Name()))
		if err1 != nil {
			return nil, err1
		}
		// ignore nocopy files
		if bytes.Contains(readFile, []byte("//skywalking:nocopy")) {
			continue
		}

		files = append(files, rewrite.NewFile(packageName, entry.Name(), string(readFile)))
	}

	rewrited, err := context.MultipleFilesWithWritten("skywalking_enhance_", baseDir, packageName, files, debugBaseDir)
	if err != nil {
		return nil, err
	}
	return rewrited, nil
}

func (i *Instrument) copyOperatorsFS(context *rewrite.Context, baseDir, packageName string) ([]string, error) {
	result := make([]string, 0)
	var debugBaseDir string
	for _, dir := range rewrite.OperatorDirs {
		entries, err := core.FS.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		files := make([]*rewrite.FileInfo, 0)
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, err1 := fs.ReadFile(core.FS, filepath.Join(dir, entry.Name()))
			if err1 != nil {
				return nil, err1
			}
			files = append(files, rewrite.NewFile(dir, entry.Name(), string(file)))
		}
		if i.compileOpts.DebugDir != "" {
			debugBaseDir = filepath.Join(i.compileOpts.DebugDir, "plugins", "core", dir)
		}

		rewrited, err := context.MultipleFilesWithWritten("skywalking_agent_core_", baseDir, filepath.Base(dir), files, debugBaseDir)
		if err != nil {
			return nil, err
		}
		result = append(result, rewrited...)
	}

	// write extra file for link the operator function
	tmpFiles, err := tools.WriteMultipleFile(baseDir, map[string]string{
		"skywalking_agent_core_linker.go": tools.ExecuteTemplate(`package {{.PackageName}}

import (
	_ "unsafe"
)

//go:linkname {{.OperatorGetLinkMethod}} {{.OperatorGetLinkMethod}}
var {{.OperatorGetLinkMethod}} func() interface{}

func init() {
	if {{.OperatorGetLinkMethod}} != nil {
		{{.OperatorGetRealMethod}} = func() {{.OperatorTypeName}} {
			tmpOp := {{.OperatorGetLinkMethod}}()
			if tmpOp == nil {
				return nil
			}
			if opVal, ok := tmpOp.({{.OperatorTypeName}}); ok {
				return opVal
			}
			return nil
		}
	}
}
`, struct {
			PackageName           string
			OperatorGetLinkMethod string
			OperatorGetRealMethod string
			OperatorTypeName      string
		}{
			PackageName:           packageName,
			OperatorGetLinkMethod: rewrite.GlobalOperatorLinkGetMethodName,
			OperatorGetRealMethod: rewrite.GlobalOperatorRealGetMethodName,
			OperatorTypeName:      rewrite.GlobalOperatorTypeName,
		}),
	})
	if err != nil {
		return nil, err
	}
	result = append(result, tmpFiles...)
	return result, nil
}

func (i *Instrument) writeDelegatorFile(ctx *rewrite.Context, basePath string) ([]string, error) {
	file := &dst.File{
		Name: dst.NewIdent("delegator"), // write to adapter temporary, it will be rewritten later
	}

	for _, enhance := range i.enhancements {
		file.Decls = append(file.Decls, enhance.BuildForDelegator()...)
	}

	ctx.SingleFile(file)

	adapterFile := filepath.Join(basePath, "skywalking_delegator.go")
	if err := tools.WriteDSTFile(adapterFile, file, nil); err != nil {
		return nil, err
	}
	return []string{adapterFile}, nil
}

func (i *Instrument) validateStructIsMatch(matcher *instrument.EnhanceMatcher, node *dst.TypeSpec, allFiles []*dst.File) bool {
	if matcher.Name != node.Name.Name {
		return false
	}
	if _, ok := node.Type.(*dst.StructType); !ok {
		return false
	}
	for _, filter := range matcher.StructFilters {
		if !filter(node, allFiles) {
			return false
		}
	}
	return true
}

func (i *Instrument) validateMethodInsMatch(matcher *instrument.EnhanceMatcher, node *dst.FuncDecl, allFiles []*dst.File) bool {
	if matcher.Name != node.Name.Name {
		return false
	}
	if matcher.Receiver != "" {
		if node.Recv == nil || len(node.Recv.List) == 0 {
			return false
		}
		var data dst.Expr
		switch t := node.Recv.List[0].Type.(type) {
		case *dst.StarExpr:
			data = t.X
		case *dst.TypeAssertExpr:
			data = t.X
		default:
			return false
		}

		if id, ok := data.(*dst.Ident); !ok {
			return false
		} else if id.Name != matcher.Receiver {
			return false
		}
	}
	for _, filter := range matcher.MethodFilters {
		if !filter(node, allFiles) {
			return false
		}
	}
	return true
}

func (i *Instrument) tryToFindThePluginVersion(opts *api.CompileOptions, ins instrument.Instrument) (string, error) {
	for _, arg := range opts.AllArgs {
		// find the go file
		if !strings.HasSuffix(arg, ".go") {
			continue
		}
		basePkg := ins.BasePackage()

		parts := strings.SplitN(arg, basePkg, 2)
		// example: github.com/gin-gonic/gin@1.1.1/gin.go
		if len(parts) != 2 {
			return "", fmt.Errorf("could not found the go version of the package %s, go file path: %s", basePkg, arg)
		}
		if !strings.HasPrefix(parts[1], "@") {
			return "", nil
		}
		firstDir := strings.Index(parts[1], "/")
		if firstDir == -1 {
			return "", fmt.Errorf("could not found the first directory index for package: %s, go file path: %s", basePkg, arg)
		}
		return parts[1][1:firstDir], nil
	}
	return "", nil
}
