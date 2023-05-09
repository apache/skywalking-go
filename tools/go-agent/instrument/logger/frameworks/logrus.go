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

package frameworks

import (
	"fmt"
	"path/filepath"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type Logrus struct {
}

func NewLogrus() *Logrus {
	return &Logrus{}
}

func (l *Logrus) Name() string {
	return "logrus"
}

func (l *Logrus) PackagePaths() map[string]*PackageConfiguration {
	return map[string]*PackageConfiguration{"github.com/sirupsen/logrus": {NeedsHelpers: true}}
}

func (l *Logrus) AutomaticBindFunctions(fun *dst.FuncDecl) string {
	// if init the output or format then the logrus would be bind
	if fun.Recv == nil || len(fun.Recv.List) != 1 || tools.GenerateTypeNameByExp(fun.Recv.List[0].Type) != "*Logger" {
		return ""
	}

	if fun.Name.Name == "SetOutput" || fun.Name.Name == "SetFormatter" {
		return rewrite.StaticMethodPrefix + "LogrusUpdateLogrusLogger(*recv_0)"
	}

	return ""
}

func (l *Logrus) GenerateExtraFiles(pkgPath, debugDir string) ([]*rewrite.FileInfo, error) {
	return []*rewrite.FileInfo{
		l.generateReWriteFile("logrus_adapt.go", debugDir),
		l.generateReWriteFile("logrus_format.go", debugDir),
	}, nil
}

func (l *Logrus) generateReWriteFile(name, debugDir string) *rewrite.FileInfo {
	file, err := FrameworkFS.ReadFile(name)
	if err != nil {
		panic(fmt.Errorf("get logrus file error: %v", err))
	}

	if debugDir == "" {
		return rewrite.NewFile("logrus", name, string(file))
	}
	return rewrite.NewFileWithDebug("logrus", name, string(file),
		filepath.Join(debugDir, "tools", "go-agent", "instrument", "logger", "frameworks"))
}

func (l *Logrus) CustomizedEnhance(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) (map[string]string, bool) {
	return nil, false
}
