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
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

func CopyGoFiles(fromFS fs.ReadDirFS, fromDir, targetDir string,
	debugInfoBuilder func(entry fs.DirEntry, file *dst.File) (*DebugInfo, error),
	peek func(file *dst.File)) ([]string, error) {
	results := make([]string, 0)
	files, err := fromFS.ReadDir(fromDir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}

		readFile, err := fs.ReadFile(fromFS, filepath.Join(fromDir, f.Name()))
		if err != nil {
			return nil, err
		}

		// ignore nocopy files
		if bytes.Contains(readFile, []byte("//skywalking:nocopy")) {
			continue
		}

		parse, err := decorator.Parse(readFile)
		if err != nil {
			return nil, err
		}
		debugInfo, err := debugInfoBuilder(f, parse)
		if err != nil {
			return nil, err
		}

		peek(parse)
		copiedFilePath := filepath.Join(targetDir, f.Name())
		if err := WriteDSTFile(copiedFilePath, parse, debugInfo); err != nil {
			return nil, err
		}
		results = append(results, copiedFilePath)
	}
	return results, nil
}
