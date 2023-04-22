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

package reporter

import (
	"html"
	"io/fs"
	"path/filepath"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

var GRPCInitFuncName = "GRPCReporterInit"

type GRPCInstrument struct {
	hasToEnhance bool
	compileOpts  *api.CompileOptions
}

func NewGRPCInstrument() *GRPCInstrument {
	return &GRPCInstrument{}
}

func (i *GRPCInstrument) CouldHandle(opts *api.CompileOptions) bool {
	i.compileOpts = opts
	return opts.Package == "github.com/apache/skywalking-go/reporter"
}

func (i *GRPCInstrument) FilterAndEdit(path string, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	if i.hasToEnhance {
		return false
	}
	i.hasToEnhance = true
	return true
}

func (i *GRPCInstrument) AfterEnhanceFile(fromPath, newPath string) error {
	return nil
}

func (i *GRPCInstrument) WriteExtraFiles(dir string) ([]string, error) {
	// copy gRPC reporter enhance files
	results := make([]string, 0)
	copiedFiles, err := tools.CopyGoFiles(core.FS, "reporter", dir, func(entry fs.DirEntry) (*tools.DebugInfo, error) {
		if i.compileOpts.DebugDir == "" {
			return nil, nil
		}
		debugPath := filepath.Join(i.compileOpts.DebugDir, "reporter", entry.Name())
		return tools.BuildDSTDebugInfo(debugPath, nil)
	}, func(file *dst.File) {
	})
	if err != nil {
		return nil, err
	}
	results = append(results, copiedFiles...)

	// generate the file for export the reporter
	file, err := i.generateReporterInitFile(dir)
	if err != nil {
		return nil, err
	}
	results = append(results, file)

	return results, nil
}

func (i *GRPCInstrument) generateReporterInitFile(dir string) (string, error) {
	return tools.WriteFile(dir, "grpc_init.go", html.UnescapeString(tools.ExecuteTemplate(`package reporter

import (
	"github.com/apache/skywalking-go/log"
	"fmt"
	"strconv"
	"os"
)

func {{.InitFuncName}}(logger log.Logger) (Reporter, error) {
	return NewGRPCReporter(logger, {{.Config.Reporter.GRPC.BackendService.ToGoStringValue}},
		WithMaxSendQueueSize({{.Config.Reporter.GRPC.MaxSendQueue.ToGoIntValue "the GRPC reporter max queue size must be number"}}))
}
`, struct {
		InitFuncName string
		Config       *config.Config
	}{
		InitFuncName: GRPCInitFuncName,
		Config:       config.GetConfig(),
	})))
}
