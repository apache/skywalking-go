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
	"go/token"
	"path/filepath"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/agentcore"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func (c *Context) Import(imp *dst.ImportSpec, cursor *dstutil.Cursor) {
	shouldRemove := false
	operatorCode := false
	// delete the original import("agent/core/xxx")
	for _, subPackageName := range OperatorDirs {
		if imp.Path.Value == fmt.Sprintf("%q", filepath.Join(agentcore.EnhanceFromBasePackage, subPackageName)) {
			shouldRemove = true
			operatorCode = true
			break
		}
	}
	// delete the same framework package import, such as the interceptor of http package("github.com/gin-gonic/gin")
	if imp.Path.Value == fmt.Sprintf("%q", c.pkgFullPath) {
		shouldRemove = true
	}

	if shouldRemove {
		realPath := imp.Path.Value[1 : len(imp.Path.Value)-1]
		subPath := filepath.Base(realPath)
		info := &rewriteImportInfo{pkgName: subPath, isAgentCore: operatorCode, ctx: c}
		if imp.Name == nil {
			c.packageImport[subPath] = info
		} else {
			c.packageImport[imp.Name.Name] = info
		}
		cursor.Delete()
	}
}

func (c *Context) AddingImportToCurrentFile(name, path string) {
	// if needs to been added package same with current package, then ignore
	if c.pkgFullPath == path {
		return
	}
	var latestImport *dst.GenDecl
	var containsPath bool
	dstutil.Apply(c.currentProcessingFile, func(cursor *dstutil.Cursor) bool {
		if gen, ok := cursor.Node().(*dst.GenDecl); ok && gen.Tok == token.IMPORT {
			latestImport = gen
			for _, spec := range gen.Specs {
				if imp, ok := spec.(*dst.ImportSpec); ok && imp.Path.Value == fmt.Sprintf("%q", path) {
					containsPath = true
				}
			}
			return false
		}
		// reduce the looping
		if cursor.Node() == c.currentProcessingFile {
			return true
		}
		return false
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})

	// if already contains the path, then ignore
	if containsPath {
		return
	}

	// if import not found, then create one and add to the file
	if latestImport == nil {
		latestImport = &dst.GenDecl{
			Tok:   token.IMPORT,
			Specs: []dst.Spec{},
		}
		c.currentProcessingFile.Decls = append([]dst.Decl{latestImport}, c.currentProcessingFile.Decls...)
	}

	// adding the import
	imp := &dst.ImportSpec{
		Path: &dst.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("%q", path),
		},
		Name: dst.NewIdent(name),
	}
	latestImport.Specs = append(latestImport.Specs, imp)
}
