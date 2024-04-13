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
	"strings"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/agentcore"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type GRPCInstrument struct {
	hasToEnhance bool
	compileOpts  *api.CompileOptions
}

func NewGRPCInstrument() *GRPCInstrument {
	return &GRPCInstrument{}
}

func (i *GRPCInstrument) CouldHandle(opts *api.CompileOptions) bool {
	i.compileOpts = opts
	return opts.Package == "github.com/apache/skywalking-go/agent/reporter"
}

func (i *GRPCInstrument) FilterAndEdit(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
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
	// copy reporter api files
	results := make([]string, 0)
	copiedFiles, err := tools.CopyGoFiles(core.FS, "reporter", dir, func(entry fs.DirEntry, f *dst.File) (*tools.DebugInfo, error) {
		if i.compileOpts.DebugDir == "" {
			return nil, nil
		}
		debugPath := filepath.Join(i.compileOpts.DebugDir, "plugins", "core", "reporter", entry.Name())
		return tools.BuildDSTDebugInfo(debugPath, nil)
	}, func(file *dst.File) {
	})
	if err != nil {
		return nil, err
	}
	results = append(results, copiedFiles...)

	// copy reporter implementations
	// Force the use of '/' delimiter on all platforms
	reporterDirName := strings.ReplaceAll(filepath.Join("reporter", "grpc"), `\`, `/`)
	copiedFiles, err = tools.CopyGoFiles(core.FS, reporterDirName, dir, func(entry fs.DirEntry, f *dst.File) (*tools.DebugInfo, error) {
		if i.compileOpts.DebugDir == "" {
			return nil, nil
		}
		debugPath := filepath.Join(i.compileOpts.DebugDir, "plugins", "core", reporterDirName, entry.Name())
		return tools.BuildDSTDebugInfo(debugPath, f)
	}, func(file *dst.File) {
		file.Name = dst.NewIdent("reporter")
		pkgUpdates := make(map[string]string)
		for _, p := range agentcore.CopiedSubPackages {
			key := strings.ReplaceAll(filepath.Join(agentcore.EnhanceFromBasePackage, p), `\`, `/`)
			val := strings.ReplaceAll(filepath.Join(agentcore.EnhanceBasePackage, p), `\`, `/`)
			pkgUpdates[key] = val
		}
		tools.ChangePackageImportPath(file, pkgUpdates)
		tools.DeletePackageImports(file, "github.com/apache/skywalking-go/plugins/core/reporter")
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
	"github.com/apache/skywalking-go/agent/core/operator"
	"fmt"
	"strconv"
	"os"
	"time"
	"strings"
)

func {{.InitFuncName}}(logger operator.LogOperator) (Reporter, error) {
	if {{.Config.Reporter.Discard.ToGoBoolValue}} {
		return NewDiscardReporter(), nil
	}
	var opts []ReporterOption
	checkIntervalVal := {{.Config.Reporter.GRPC.CheckInterval.ToGoIntValue "the GRPC reporter check interval must be number"}}
	opts = append(opts, WithCheckInterval(time.Second * time.Duration(checkIntervalVal)))
	opts = append(opts, WithMaxSendQueueSize({{.Config.Reporter.GRPC.MaxSendQueue.ToGoIntValue "the GRPC reporter max queue size must be number"}}))
	opts = append(opts, WithAuthentication({{.Config.Reporter.GRPC.Authentication.ToGoStringValue}}))
	cdsFetchIntervalVal := {{.Config.Reporter.GRPC.CDSFetchInterval.ToGoIntValue "the GRPC reporter max queue size must be number"}}
	opts = append(opts, WithCDS(time.Second * time.Duration(cdsFetchIntervalVal)))

	if {{.Config.Reporter.GRPC.TLS.Enable.ToGoBoolValue}} {
		tc, err := generateTLSCredential({{.Config.Reporter.GRPC.TLS.CAPath.ToGoStringValue}}, 
			{{.Config.Reporter.GRPC.TLS.ClientKeyPath.ToGoStringValue}},
			{{.Config.Reporter.GRPC.TLS.ClientCertChainPath.ToGoStringValue}},
			{{.Config.Reporter.GRPC.TLS.InsecureSkipVerify.ToGoBoolValue}})
		if err != nil {
			panic(fmt.Sprintf("generate go agent tls credential error: %v", err))
		}
		opts = append(opts, WithTransportCredentials(tc))
	}

	return NewGRPCReporter(logger, {{.Config.Reporter.GRPC.BackendService.ToGoStringValue}}, opts...)
}
`, struct {
		InitFuncName string
		Config       *config.Config
	}{
		InitFuncName: consts.GRPCInitFuncName,
		Config:       config.GetConfig(),
	})))
}
