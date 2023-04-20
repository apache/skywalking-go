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

// MultipleFilesWithWritten for rewrite all operator/interceptor files
func (c *Context) MultipleFilesWithWritten(writeFileNamePrefix, targetDir, fromPackage string,
	originalFiles []*FileInfo, debugBaseDir string) ([]string, error) {
	result := make([]string, 0)

	for _, f := range originalFiles {
		parseFile, err := decorator.ParseFile(nil, f.FileName, f.FileData, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		var debugInfo *tools.DebugInfo
		if debugBaseDir != "" {
			debugInfo, err = tools.BuildDSTDebugInfo(filepath.Join(debugBaseDir, f.FileName), parseFile)
			if err != nil {
				return nil, err
			}
		}

		c.processSingleFile(parseFile, fromPackage)
		targetPath := filepath.Join(targetDir,
			fmt.Sprintf("%s%s_%s", writeFileNamePrefix, f.PackageName, filepath.Base(f.FileName)))
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
	c.currentPackageTitle = c.titleCase.String(fromPackage)
	file.Name.Name = c.targetPackage
	dstutil.Apply(file, func(cursor *dstutil.Cursor) bool {
		switch n := cursor.Node().(type) {
		case *dst.FuncDecl:
			c.Func(n, cursor)
		case *dst.ImportSpec:
			c.Import(n, cursor)
		case *dst.TypeSpec:
			c.Type(n)
		case *dst.ValueSpec:
			c.Var(n)
		default:
			return true
		}
		return false
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})

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
