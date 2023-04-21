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
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

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
	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()

	fset, af, err := decorator.RestoreFile(file)
	if err != nil {
		return err
	}

	if debug != nil {
		return writeDSTFileWithDebug(fset, af, debug, output)
	}
	return printer.Fprint(output, fset, af)
}

func writeDSTFileWithDebug(fset *token.FileSet, file *ast.File, debug *DebugInfo, output *os.File) error {
	var changeInfo *dstFilePathChangeInfo
	if !debug.CheckOldLine {
		changeInfo = &dstFilePathChangeInfo{
			oldDebugPath: debug.FilePath,
			oldDebugLine: 1,
			newDebugLine: 1,
		}
		if _, err := output.WriteString(fmt.Sprintf("//line %s:%d\n", debug.FilePath, debug.Line)); err != nil {
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
	if pos == 0 {
		if len(file.Decls) > 0 {
			return fset.Position(file.Decls[0].Pos()).Line, nil
		}
		return 1, nil
	}
	importEndLine := fset.Position(pos).Line
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
		if trimed == "" || trimed == ")" {
			continue
		}
		return lineNumber, nil
	}
}
