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
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

var packageImportExp = regexp.MustCompile(`^(\S+\s+)?"(.+)"$`)

func ChangePackageImportPath(file dst.Node, pkgChanges map[string]string) {
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		if n, ok := cursor.Node().(*dst.ImportSpec); ok {
			for originalPkg, targetPkg := range pkgChanges {
				sprintf := fmt.Sprintf("%q", originalPkg)
				if n.Path.Value == sprintf {
					n.Path.Value = fmt.Sprintf("%q", targetPkg)
				}
			}
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
}

func DeletePackageImports(file dst.Node, imports ...string) {
	containsDeletedImport := false
	deletedPackages := make(map[string]string)
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		switch n := cursor.Node().(type) {
		case *dst.ImportSpec:
			for _, pkg := range imports {
				if n.Path.Value == fmt.Sprintf("%q", pkg) {
					containsDeletedImport = true
					cursor.Delete()

					if n.Name != nil {
						deletedPackages[n.Name.Name] = pkg
					} else {
						deletedPackages[filepath.Base(pkg)] = pkg
					}
				}
			}
			return false
		case *dst.SelectorExpr:
			pkgRefName, ok := n.X.(*dst.Ident)
			if !ok {
				return true
			}
			if _, ok := deletedPackages[pkgRefName.Name]; ok {
				RemovePackageRef(cursor.Parent(), n, -1)
			}
		case *dst.CaseClause:
			for i, d := range n.List {
				if sel, ok := d.(*dst.SelectorExpr); ok {
					pkgRefName, ok := sel.X.(*dst.Ident)
					if !ok {
						return true
					}
					if _, ok := deletedPackages[pkgRefName.Name]; ok {
						RemovePackageRef(n, sel, i)
					}
				}
			}
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})

	if containsDeletedImport {
		RemoveImportDefineIfNoPackage(file)
	}
}

func RemovePackageRef(parent dst.Node, current *dst.SelectorExpr, inx int) {
	switch p := parent.(type) {
	case *dst.Field:
		p.Type = dst.NewIdent(current.Sel.Name)
	case *dst.Ellipsis:
		p.Elt = dst.NewIdent(current.Sel.Name)
	case *dst.StarExpr:
		p.X = dst.NewIdent(current.Sel.Name)
	case *dst.TypeAssertExpr:
		p.Type = dst.NewIdent(current.Sel.Name)
	case *dst.CompositeLit:
		p.Type = dst.NewIdent(current.Sel.Name)
	case *dst.ArrayType:
		p.Elt = dst.NewIdent(current.Sel.Name)
	case *dst.CallExpr:
		p.Fun = dst.NewIdent(current.Sel.Name)
	case *dst.KeyValueExpr:
		p.Value = dst.NewIdent(current.Sel.Name)
	case *dst.AssignStmt:
		p.Rhs = []dst.Expr{dst.NewIdent(current.Sel.Name)}
	case *dst.CaseClause:
		p.List[inx] = dst.NewIdent(current.Sel.Name)
	}
}

func RemoveImportDefineIfNoPackage(file dst.Node) {
	// remove the import decl if empty
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		if decl, ok := cursor.Node().(*dst.GenDecl); ok && decl.Tok == token.IMPORT && len(decl.Specs) == 0 {
			cursor.Delete()
			return false
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
}

type DebugInfo struct {
	FilePath     string
	Line         int
	CheckOldLine bool
}

func BuildDSTDebugInfo(srcPath string, file *dst.File) (*DebugInfo, error) {
	result := &DebugInfo{FilePath: srcPath}
	if file != nil {
		fset, f, err := decorator.RestoreFile(file)
		if err != nil {
			return nil, err
		}
		originalFile, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, err
		}
		line, err := findFirstNoImportLocation(fset, f, bytes.NewBuffer(originalFile))
		if err != nil {
			return nil, err
		}
		result.Line = line
		result.CheckOldLine = true
	} else {
		result.Line = 1
		result.CheckOldLine = false
	}

	return result, nil
}

func WriteDSTFile(path string, file *dst.File, debug *DebugInfo) error {
	if runtime.GOOS == consts.WindowsGOOS {
		path = strings.ReplaceAll(path, `/`, `\`)
	}

	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()

	content, err := GenerateDSTFileContent(file, debug)
	if err != nil {
		return err
	}

	if _, err = output.WriteString(content); err != nil {
		return err
	}
	return nil
}

func GenerateDSTFileContent(file *dst.File, debug *DebugInfo) (string, error) {
	var buf bytes.Buffer
	writer := io.Writer(&buf)

	fset, af, err := decorator.RestoreFile(file)
	if err != nil {
		return "", err
	}

	if debug != nil {
		if err1 := writeDSTFileWithDebug(fset, af, debug, writer); err1 != nil {
			return "", err1
		}
		return buf.String(), nil
	}

	if err := printer.Fprint(writer, fset, af); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func BuildFuncIdentity(pkgPath string, node *dst.FuncDecl) string {
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

type ImportAnalyzer struct {
	imports     map[string]map[string]*dst.ImportSpec
	usedImports map[string]*dst.ImportSpec
}

func CreateImportAnalyzer() *ImportAnalyzer {
	return &ImportAnalyzer{
		imports:     make(map[string]map[string]*dst.ImportSpec),
		usedImports: make(map[string]*dst.ImportSpec)}
}

func (i *ImportAnalyzer) AnalyzeFileImports(filePath string, f dst.Node) {
	imports := make(map[string]*dst.ImportSpec)
	i.imports[filePath] = imports
	dstutil.Apply(f, func(cursor *dstutil.Cursor) bool {
		importSpec, ok := cursor.Node().(*dst.ImportSpec)
		if !ok {
			return true
		}
		var pkgName = filepath.Base(importSpec.Path.Value)
		if importSpec.Name != nil {
			pkgName = importSpec.Name.Name
		}
		imports[strings.Trim(pkgName, "\"")] = importSpec
		return false
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
}

func (i *ImportAnalyzer) AnalyzeNeedsImports(filePath string, fields *dst.FieldList) {
	if fields == nil || len(fields.List) == 0 {
		return
	}

	for _, f := range fields.List {
		i.analyzeFieldImport(filePath, f.Type)
	}
}

func (i *ImportAnalyzer) analyzeFieldImport(filePath string, exp dst.Expr) {
	switch n := exp.(type) {
	case *dst.Ident:
		return
	case *dst.SelectorExpr:
		pkgRefName, ok := n.X.(*dst.Ident)
		if !ok {
			return
		}
		imports := i.imports[filePath]
		if imports == nil {
			return
		}
		spec := imports[pkgRefName.Name]
		if spec == nil {
			return
		}
		i.usedImports[pkgRefName.Name] = spec
	case *dst.Ellipsis:
		i.analyzeFieldImport(filePath, n.Elt)
	case *dst.ArrayType:
		i.analyzeFieldImport(filePath, n.Elt)
	case *dst.StarExpr:
		i.analyzeFieldImport(filePath, n.X)
	case *dst.ChanType:
		i.analyzeFieldImport(filePath, n.Value)
	}
}

func (i *ImportAnalyzer) AppendUsedImports(decl *dst.GenDecl) {
	if decl.Tok != token.IMPORT {
		return
	}
	for _, spec := range i.usedImports {
		found := false
		for _, existingSpec := range decl.Specs {
			if existingSpec.(*dst.ImportSpec).Path.Value == spec.Path.Value {
				found = true
				break
			}
		}
		if !found {
			decl.Specs = append(decl.Specs, dst.Clone(spec).(*dst.ImportSpec))
		}
	}
}

func writeDSTFileWithDebug(fset *token.FileSet, file *ast.File, debug *DebugInfo, output io.Writer) error {
	var changeInfo *dstFilePathChangeInfo
	if !debug.CheckOldLine {
		changeInfo = &dstFilePathChangeInfo{
			oldDebugPath: debug.FilePath,
			oldDebugLine: 1,
			newDebugLine: 1,
		}
		if _, err := fmt.Fprintf(output, "//line %s:%d\n", debug.FilePath, debug.Line); err != nil {
			return err
		}
		if err := printer.Fprint(output, fset, file); err != nil {
			return err
		}
		return nil
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, fset, file); err != nil {
		return err
	}
	newPosition, err := findFirstNoImportLocation(fset, file, bytes.NewBuffer(buffer.Bytes()))
	if err != nil {
		return err
	}
	changeInfo = &dstFilePathChangeInfo{
		oldDebugPath: debug.FilePath,
		oldDebugLine: debug.Line,
		newDebugLine: newPosition,
	}

	lineCount := 1
	alreadyChange := false
	for {
		line, err := buffer.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && !alreadyChange {
				return fmt.Errorf("rewrite file line number failure: %v", err)
			}
			break
		}

		if lineCount == changeInfo.newDebugLine {
			line = []byte(fmt.Sprintf("//line %s:%d\n%s", debug.FilePath, changeInfo.oldDebugLine, line))
			alreadyChange = true
		}

		if _, e := output.Write(line); e != nil {
			return err
		}

		lineCount++
	}
	return nil
}

type dstFilePathChangeInfo struct {
	oldDebugPath string
	oldDebugLine int
	newDebugLine int
}

func findFirstNoImportLocation(fset *token.FileSet, file *ast.File, fileContent *bytes.Buffer) (int, error) {
	var pos token.Pos
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Tok == token.IMPORT {
				pos = genDecl.End()
				continue
			}
		}
		break
	}
	importEndLine := fset.Position(pos).Line
	if pos == 0 {
		if len(file.Decls) == 0 {
			return 1, nil
		}
		importEndLine = fset.Position(file.Decls[0].Pos()).Line
	}
	lineNumber := 0
	for {
		line, err := fileContent.ReadBytes('\n')
		if err != nil {
			return 0, err
		}
		lineNumber++
		if lineNumber < importEndLine {
			continue
		}
		trimed := strings.TrimSpace(string(line))
		if trimed == "" || trimed == ")" ||
			(strings.HasPrefix(trimed, "import ")) ||
			(packageImportExp.MatchString(trimed)) {
			continue
		}
		return lineNumber, nil
	}
}
