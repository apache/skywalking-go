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
	"fmt"
	"go/printer"
	"os"

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

func WriteDSTFile(path, srcPath string, file *dst.File) error {
	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()
	if srcPath != "" {
		if _, err = output.WriteString(fmt.Sprintf("//line %s:1\n", srcPath)); err != nil {
			return err
		}
	}
	fset, af, err := decorator.RestoreFile(file)
	if err != nil {
		return err
	}
	if err := printer.Fprint(output, fset, af); err != nil {
		return err
	}
	return nil
}
