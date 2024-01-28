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
	"html"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

var (
	ProjectBasePackage      = "github.com/apache/skywalking-go/"
	EnhanceBasePackage      = ProjectBasePackage + "agent/core"
	EnhanceFromBasePackage  = ProjectBasePackage + "plugins/core"
	ReporterFromBasePackage = "reporter"
	ReporterBasePackage     = "agent/reporter"

	CopiedBasePackage = `skywalking-go(@[\d\w\.\-]+)?\/agent\/core`
	CopiedSubPackages = []string{"", "tracing", "operator"}
)

type Instrument struct {
	hasCopyPath  bool
	needsCopyDir string
	compileOpts  *api.CompileOptions
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	i.compileOpts = opts
	return strings.HasPrefix(opts.Package, EnhanceBasePackage)
}

func (i *Instrument) FilterAndEdit(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	if i.hasCopyPath {
		return false
	}
	targetDir := filepath.Dir(path)
	for _, sub := range CopiedSubPackages {
		if regexp.MustCompile(filepath.Join(CopiedBasePackage, sub) + "$").MatchString(targetDir) {
			i.needsCopyDir = sub
			i.hasCopyPath = true
			return true
		}
	}
	return false
}

func (i *Instrument) AfterEnhanceFile(fromPath, newPath string) error {
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
	}

	pkgUpdates := make(map[string]string)
	for _, p := range CopiedSubPackages {
		pkgUpdates[filepath.Join(EnhanceFromBasePackage, p)] = filepath.Join(EnhanceBasePackage, p)
	}
	pkgUpdates[filepath.Join(EnhanceFromBasePackage, ReporterFromBasePackage)] = filepath.Join(ProjectBasePackage, ReporterBasePackage)
	copiedFiles, err := tools.CopyGoFiles(core.FS, sub, dir, i.buildDSTDebugInfo, func(file *dst.File) {
		tools.ChangePackageImportPath(file, pkgUpdates)
	})
	if err != nil {
		return nil, err
	}
	results = append(results, copiedFiles...)

	// write extra file to link the operator and TLS methods
	if sub == "." {
		file, err := i.writeLinkerFile(dir)
		if err != nil {
			return nil, err
		}
		results = append(results, file)

		file1, err := i.writeTracerInitLink(dir)
		if err != nil {
			return nil, err
		}
		results = append(results, file1)
	}

	return results, nil
}

func (i *Instrument) buildDSTDebugInfo(entry fs.DirEntry, _ *dst.File) (*tools.DebugInfo, error) {
	if i.compileOpts.DebugDir == "" {
		return nil, nil
	}
	debugPath := filepath.Join(i.compileOpts.DebugDir, "plugins", "core", entry.Name())
	return tools.BuildDSTDebugInfo(debugPath, nil)
}

func (i *Instrument) writeTracerInitLink(dir string) (string, error) {
	return tools.WriteFile(dir, "tracer_init.go", html.UnescapeString(tools.ExecuteTemplate(`package core

import (
	"github.com/apache/skywalking-go/agent/reporter"
	"github.com/apache/skywalking-go/agent/core/operator"
	"fmt"
	"os"
	"strconv"
	_ "unsafe"
)

//go:linkname {{.GetGlobalLoggerLinkMethod}} {{.GetGlobalLoggerLinkMethod}}
var {{.GetGlobalLoggerLinkMethod}} func() interface{}

func (t *Tracer) InitTracer(extend map[string]interface{}) {
	rep, err := reporter.{{.GRPCReporterFuncName}}(t.Log)
	if err != nil {
		t.Log.Errorf("cannot initialize the reporter: %v", err)
		return
	}
	entity := NewEntity({{.Config.Agent.ServiceName.ToGoStringValue}}, {{.Config.Agent.InstanceEnvName.ToGoStringValue}})
	samp := NewDynamicSampler({{.Config.Agent.Sampler.ToGoFloatValue "loading the agent sampler error"}}, t)
	meterCollectInterval := {{.Config.Agent.Meter.CollectInterval.ToGoIntValue "loading the agent meter interval error"}}
	var logger operator.LogOperator
	if {{.GetGlobalLoggerLinkMethod}} != nil {
		if l, ok := {{.GetGlobalLoggerLinkMethod}}().(operator.LogOperator); ok &&  l != nil {
			logger = l
		}
	}
	correlation := &CorrelationConfig{
		MaxKeyCount : {{.Config.Agent.Correlation.MaxKeyCount.ToGoIntValue "loading the agent correlation maxKeyCount error"}},
		MaxValueSize : {{.Config.Agent.Correlation.MaxValueSize.ToGoIntValue "loading the agent correlation maxValueSize error"}},
	}
	ignoreSuffixStr := {{.Config.Agent.IgnoreSuffix.ToGoStringValue}}
	if err := t.Init(entity, rep, samp, logger, meterCollectInterval, correlation, ignoreSuffixStr); err != nil {
		t.Log.Errorf("cannot initialize the SkyWalking Tracer: %v", err)
	}
}`, struct {
		GRPCReporterFuncName      string
		GetGlobalLoggerLinkMethod string
		Config                    *config.Config
	}{
		GRPCReporterFuncName:      consts.GRPCInitFuncName,
		GetGlobalLoggerLinkMethod: consts.GlobalLoggerGetMethodName,
		Config:                    config.GetConfig(),
	})))
}

func (i *Instrument) writeLinkerFile(dir string) (string, error) {
	return tools.WriteFile(dir, "runtime_linker.go", tools.ExecuteTemplate(`package core

import (
	_ "unsafe"
)

//go:linkname {{.TLSGetLinkMethod}} {{.TLSGetLinkMethod}}
var {{.TLSGetLinkMethod}} func() interface{}

//go:linkname {{.TLSSetLinkMethod}} {{.TLSSetLinkMethod}}
var {{.TLSSetLinkMethod}} func(interface{})

//go:linkname {{.SetGlobalOperatorLinkMethod}} {{.SetGlobalOperatorLinkMethod}}
var {{.SetGlobalOperatorLinkMethod}} func(interface{}) 

//go:linkname {{.GetGlobalOperatorLinkMethod}} {{.GetGlobalOperatorLinkMethod}}
var {{.GetGlobalOperatorLinkMethod}} func() interface{}

//go:linkname {{.GetGoroutineIDLinkMethod}} {{.GetGoroutineIDLinkMethod}}
var {{.GetGoroutineIDLinkMethod}} func() int64

//go:linkname {{.GetInitNotifyLinkMethod}} {{.GetInitNotifyLinkMethod}}
var {{.GetInitNotifyLinkMethod}} func() []func()

//go:linkname {{.MetricsObtainMethodName}} {{.MetricsObtainMethodName}}
var {{.MetricsObtainMethodName}} func() ([]interface{}, []func())

func init() {
	if {{.TLSGetLinkMethod}} != nil && {{.TLSSetLinkMethod}} != nil {
		GetGLS = {{.TLSGetLinkMethod}}
		SetGLS = {{.TLSSetLinkMethod}}
	}
	if {{.GetGoroutineIDLinkMethod}} != nil {
		GetGoID = {{.GetGoroutineIDLinkMethod}}
	}
	if {{.SetGlobalOperatorLinkMethod}} != nil && {{.GetGlobalOperatorLinkMethod}} != nil {
		SetGlobalOperator = {{.SetGlobalOperatorLinkMethod}}
		GetGlobalOperator = {{.GetGlobalOperatorLinkMethod}}
		SetGlobalOperator(newTracer())	// setting the global tracer when init the agent core
	}
	if {{.GetInitNotifyLinkMethod}} != nil {
		GetInitNotify = {{.GetInitNotifyLinkMethod}}
	}
	if {{.MetricsObtainMethodName}} != nil {
		MetricsObtain = {{.MetricsObtainMethodName}}
	}
}
`, struct {
		TLSGetLinkMethod            string
		TLSSetLinkMethod            string
		SetGlobalOperatorLinkMethod string
		GetGlobalOperatorLinkMethod string
		GetGoroutineIDLinkMethod    string
		GetInitNotifyLinkMethod     string
		MetricsObtainMethodName     string
	}{
		TLSGetLinkMethod:            consts.TLSGetMethodName,
		TLSSetLinkMethod:            consts.TLSSetMethodName,
		SetGlobalOperatorLinkMethod: consts.GlobalTracerSetMethodName,
		GetGlobalOperatorLinkMethod: consts.GlobalTracerGetMethodName,
		GetGoroutineIDLinkMethod:    consts.CurrentGoroutineIDGetMethodName,
		GetInitNotifyLinkMethod:     consts.GlobalTracerInitGetNotifyMethodName,
		MetricsObtainMethodName:     consts.MetricsObtainMethodName,
	}))
}
