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

package rewrite

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"

	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

type FileInfo struct {
	// the original file path, for debugging
	OriginalFilePath string
	DebugBaseDir     string

	PackageName string
	FileName    string
	FileData    string
}

type DebugInfo struct {
	BaseDir string
}

func NewFile(packageName, fileName, data string) *FileInfo {
	return &FileInfo{PackageName: packageName, FileName: fileName, FileData: data}
}

func NewFileWithDebug(packageName, fileName, data, debugBaseDir string) *FileInfo {
	return &FileInfo{PackageName: packageName, FileName: fileName, FileData: data, DebugBaseDir: debugBaseDir}
}

// MultipleFilesWithWritten for rewrite all operator/interceptor files
func (c *Context) MultipleFilesWithWritten(writeFileNamePrefix, targetDir, fromPackage string,
	originalFiles []*FileInfo) ([]string, error) {
	result := make([]string, 0)
	c.currentPackageTitle = c.titleCase.String(fromPackage)

	// parse all files
	files := make(map[*FileInfo]*dst.File)
	for _, f := range originalFiles {
		parseFile, err := decorator.ParseFile(nil, f.FileName, f.FileData, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files[f] = parseFile
	}

	// register all top level vars, encase it cannot be found
	c.rewriteTopLevelVarFirst(files)

	var err error
	for f, parseFile := range files {
		c.processSingleFile(parseFile, fromPackage)
		targetPath := filepath.Join(targetDir,
			fmt.Sprintf("%s%s_%s", writeFileNamePrefix, f.PackageName, filepath.Base(f.FileName)))

		var debugInfo *tools.DebugInfo
		if f.DebugBaseDir != "" {
			debugInfo, err = tools.BuildDSTDebugInfo(filepath.Join(f.DebugBaseDir, f.FileName), parseFile)
			if err != nil {
				return nil, err
			}
		}
		if err := tools.WriteDSTFile(targetPath, parseFile, debugInfo); err != nil {
			return nil, err
		}
		result = append(result, targetPath)
	}
	return result, nil
}

// SingleFile rewrite single file in memory, not write the file
func (c *Context) SingleFile(file *dst.File) {
	c.processSingleFile(file, c.targetPackage)
}

func (c *Context) processSingleFile(file *dst.File, fromPackage string) {
	c.currentProcessingFile = file
	c.currentPackageTitle = c.titleCase.String(fromPackage)
	file.Name.Name = c.targetPackage
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		switch n := cursor.Node().(type) {
		case *dst.FuncDecl:
			c.Func(n, cursor)
		case *dst.ImportSpec:
			c.Import(n, cursor)
		case *dst.TypeSpec:
			c.Type(n, cursor, false)
		case *dst.ValueSpec:
			c.Var(n, false)
		default:
			return true
		}
		return false
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})

	// remove the import decl if empty
	tools.RemoveImportDefineIfNoPackage(file)
}

func (c *Context) rewriteTopLevelVarFirst(files map[*FileInfo]*dst.File) {
	for _, f := range files {
		dstutil.Apply(f, func(cursor *dstutil.Cursor) bool {
			switch n := cursor.Node().(type) {
			case *dst.FuncDecl:
			case *dst.ImportSpec:
			case *dst.TypeSpec:
				c.Type(n, cursor, true)
			case *dst.GenDecl:
				if n.Tok == token.VAR && cursor.Parent() == f {
					for _, spec := range n.Specs {
						if valueSpec, ok := spec.(*dst.ValueSpec); ok {
							c.Var(valueSpec, true)
						}
					}
				}
			default:
				return true
			}
			return false
		}, func(cursor *dstutil.Cursor) bool {
			return true
		})
	}
}
