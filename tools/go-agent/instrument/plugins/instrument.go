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
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
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

	importAnalyzer *tools.ImportAnalyzer
}

func NewInstrument() *Instrument {
	return &Instrument{
		importAnalyzer: tools.CreateImportAnalyzer(),
	}
}

type Enhance interface {
	PackageName() string
	BuildImports(decl *dst.GenDecl)
	BuildForDelegator() []dst.Decl
	ReplaceFileContent(path, content string) string
}

func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	excludePlugins := config.GetConfig().Plugin.Excluded.GetListStringResult()
	excludePluginMap := make(map[string]bool, len(excludePlugins))
	for _, v := range excludePlugins {
		excludePluginMap[v] = true
	}
	for _, ins := range instruments {
		// exclude the plugin at the compile phase if it's ignored
		if excludePluginMap[ins.Name()] {
			logrus.Infof("plugin is exclude: %s", ins.Name())
			continue
		}
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

func (i *Instrument) FilterAndEdit(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	switch n := cursor.Node().(type) {
	case *dst.TypeSpec:
		for _, filter := range i.structFilters {
			if !(i.verifyPackageIsMatch(path, filter) && i.validateStructIsMatch(filter.At, n, allFiles)) {
				continue
			}
			i.enhanceStruct(i.realInst, filter, n, path)
			tools.LogWithStructEnhance(i.compileOpts.Package, n.Name.Name, "", "adding enhanced instance field")
			return true
		}
	case *dst.FuncDecl:
		for _, filter := range i.methodFilters {
			if !(i.verifyPackageIsMatch(path, filter) && i.validateMethodInsMatch(filter.At, n, allFiles)) {
				continue
			}
			i.importAnalyzer.AnalyzeFileImports(path, curFile)
			i.enhanceMethod(i.realInst, filter, n, path)
			var receiver string
			if n.Recv != nil && len(n.Recv.List) > 0 {
				receiver = tools.GenerateTypeNameByExp(n.Recv.List[0].Type)
			}
			tools.LogWithMethodEnhance(i.compileOpts.Package, receiver, n.Name.Name, "adding enhanced method")
			return true
		}
	}
	return false
}

func (i *Instrument) enhanceStruct(_ instrument.Instrument, p *instrument.Point, typeSpec *dst.TypeSpec, _ string) {
	enhance := NewInstanceEnhance(typeSpec, i.compileOpts.Package, p)
	enhance.EnhanceField()
	i.enhancements = append(i.enhancements, enhance)
}

func (i *Instrument) enhanceMethod(inst instrument.Instrument, matcher *instrument.Point, funcDecl *dst.FuncDecl, path string) {
	enhance := NewMethodEnhance(inst, matcher, funcDecl, path, i.importAnalyzer)
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

	packageName := i.enhancements[0].PackageName()
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
	} else {
		subPkgPath = subPkgPath[1:]
	}

	var debugBaseDir string
	if i.compileOpts.DebugDir != "" {
		pathBuilder := filepath.Join(i.compileOpts.DebugDir, "plugins", i.realInst.Name())
		if subIns, ok := i.realInst.(instrument.SourceCodeDetector); ok {
			pathBuilder = filepath.Join(pathBuilder, subIns.PluginSourceCodePath())
		}
		debugBaseDir = filepath.Join(pathBuilder, subPkgPath)
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
		// if the file contains native structures, then added for ignore rewrite
		if bytes.Contains(readFile, []byte("//skywalking:native")) {
			if e := context.IncludeNativeFiles(string(readFile)); e != nil {
				return nil, e
			}
			continue
		}

		files = append(files, rewrite.NewFileWithDebug(packageName, entry.Name(), string(readFile), debugBaseDir))
	}

	rewrited, err := context.MultipleFilesWithWritten("skywalking_enhance_", baseDir, packageName, files)
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
		if i.compileOpts.DebugDir != "" {
			debugBaseDir = filepath.Join(i.compileOpts.DebugDir, "plugins", "core", dir)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), "_test.go") || strings.HasSuffix(entry.Name(), "_test_base.go") {
				continue
			}
			file, err1 := fs.ReadFile(core.FS, filepath.Join(dir, entry.Name()))
			if err1 != nil {
				return nil, err1
			}

			var rewriteFile *rewrite.FileInfo
			if debugBaseDir != "" {
				rewriteFile = rewrite.NewFileWithDebug(dir, entry.Name(), string(file), debugBaseDir)
			} else {
				rewriteFile = rewrite.NewFile(dir, entry.Name(), string(file))
			}
			files = append(files, rewriteFile)
		}

		rewrited, err := context.MultipleFilesWithWritten("skywalking_agent_core_", baseDir, filepath.Base(dir), files)
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
			OperatorGetLinkMethod: consts.GlobalTracerGetMethodName,
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

	// append header
	importsHeader := &dst.GenDecl{Tok: token.IMPORT}
	for _, e := range i.enhancements {
		e.BuildImports(importsHeader)
	}
	file.Decls = append(file.Decls, importsHeader)

	// append other decls
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
		var name = tools.GenerateTypeNameByExp(node.Recv.List[0].Type)
		return name == matcher.Receiver
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

		// example: github.com/gin-gonic/gin@1.1.1/gin.go
		_, afterPkg, found := strings.Cut(arg, basePkg)
		if !found {
			return "", fmt.Errorf("could not found the go version of the package %s, go file path: %s", basePkg, arg)
		}

		if !strings.HasPrefix(afterPkg, "@") {
			return "", nil
		}
		version, _, foundDir := strings.Cut(afterPkg, "/")
		if !foundDir {
			return "", fmt.Errorf("could not found the first directory index for package: %s, go file path: %s", basePkg, arg)
		}
		return version[1:], nil
	}
	return "", nil
}
