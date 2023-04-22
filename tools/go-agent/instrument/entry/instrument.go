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

package entry

import (
	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/framework/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type Instrument struct {
	hasFound bool
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	return opts.Package == "main"
}

func (i *Instrument) FilterAndEdit(path string, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	if i.hasFound {
		return false
	}
	i.hasFound = true
	return true
}

func (i *Instrument) AfterEnhanceFile(fromPath, newPath string) error {
	return nil
}

func (i *Instrument) WriteExtraFiles(dir string) ([]string, error) {
	file, err := tools.WriteFile(dir, "skywalking_init.go", tools.ExecuteTemplate(`package main

import (
	_ "unsafe"
)

//go:linkname {{.GetGlobalOperatorLinkMethod}} {{.GetGlobalOperatorLinkMethod}}
var {{.GetGlobalOperatorLinkMethod}} func() interface{}

type skywalkingTracerInitiator interface {
	InitTracer(map[string]interface{})
}

func init() {
	if {{.GetGlobalOperatorLinkMethod}} != nil {
		op := {{.GetGlobalOperatorLinkMethod}}()
		if op == nil {
			return
		}
		tracer, ok := op.(skywalkingTracerInitiator)
		if !ok {
			return
		}
		tracer.InitTracer(nil)
	}
}
`, struct {
		GetGlobalOperatorLinkMethod string
		Config                      *config.Config
	}{
		GetGlobalOperatorLinkMethod: rewrite.GlobalOperatorLinkGetMethodName,
		Config:                      config.GetConfig(),
	}))
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	result = append(result, file)
	return result, nil
}
