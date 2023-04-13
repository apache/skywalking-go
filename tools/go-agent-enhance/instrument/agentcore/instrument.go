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

package agentcore

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/apache/skywalking-go/plugins/core"

	"github.com/apache/skywalking-go/tools/go-agent-enhance/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent-enhance/instrument/runtime"
	"github.com/apache/skywalking-go/tools/go-agent-enhance/tools"

	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

var (
	EnhanceBasePackage     = "github.com/apache/skywalking-go/agent/core"
	EnhanceFromBasePackage = "github.com/apache/skywalking-go/plugins/core"

	CopiedBasePackage = "skywalking-go/agent/core"
	CopiedSubPackages = []string{"", "tracing"}
)

type Instrument struct {
	hasCopyPath  bool
	needsCopyDir string
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	return strings.HasPrefix(opts.Package, EnhanceBasePackage)
}

func (i *Instrument) FilterAndEdit(path string, cursor *dstutil.Cursor) bool {
	if i.hasCopyPath {
		return false
	}
	targetDir := filepath.Dir(path)
	for _, sub := range CopiedSubPackages {
		if strings.HasSuffix(targetDir, filepath.Join(CopiedBasePackage, sub)) {
			i.needsCopyDir = sub
			i.hasCopyPath = true
			return true
		}
	}
	return false
}

func (i *Instrument) AfterEnhanceFile(path string) error {
	return nil
}

func (i *Instrument) WriteExtraFiles(dir string) ([]string, error) {
	if !i.hasCopyPath {
		return nil, nil
	}
	sub := i.needsCopyDir
	results := make([]string, 0)
	if sub == "" {
		sub = "."
		// append the context adapter if is root package
		tmp, err := tools.WriteMultipleFile(dir, map[string]string{
			"adapter_context.go": tools.ExecuteTemplate(`package core

import (
	_ "unsafe"
)

//go:linkname {{.}} {{.}}
var {{.}} = TaskTracingContextSnapshot
`, runtime.TLSTakeSnapshotMethodName),
		})
		if err != nil {
			return nil, err
		}
		results = append(results, tmp...)
	}
	files, err := core.FS.ReadDir(sub)
	if err != nil {
		return nil, err
	}

	pkgUpdates := make(map[string]string)
	for _, p := range CopiedSubPackages {
		pkgUpdates[filepath.Join(EnhanceFromBasePackage, p)] = filepath.Join(EnhanceBasePackage, p)
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}

		readFile, err := fs.ReadFile(core.FS, filepath.Join(sub, f.Name()))
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

		tools.ChangePackageImportPath(parse, pkgUpdates)
		copiedFilePath := filepath.Join(dir, f.Name())
		if err := tools.WriteDSTFile(copiedFilePath, "", parse); err != nil {
			return nil, err
		}
		results = append(results, copiedFilePath)
	}

	return results, nil
}
