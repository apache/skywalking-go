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
	"embed"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// FrameworkFS export the templates of logger
//
//go:embed *
var FrameworkFS embed.FS

// ChangeLogger change the agent logger
var ChangeLogger = func(i operator.LogOperator) {
}

// LogTracingContextEnable check log the tracing context enable
var LogTracingContextEnable = func() bool {
	return false
}

// LogTracingContextKey get the tracing context key
// nolint
var LogTracingContextKey = func() string {
	return "SW_CTX"
}

// GetLogContextString get the log context string
func GetLogContextString() string {
	return ""
}

type PackageConfiguration struct {
	// needs to generate the operator helpers
	NeedsHelpers bool
}

type LogFramework interface {
	// Name of the framework
	Name() string
	// PackagePaths of the framework, define which package needs to be instrument
	PackagePaths() map[string]*PackageConfiguration
	// AutomaticBindFunctions if the filtered method invoke and log type is automatic detect,
	// then when the method invoke, the log type would be current framework
	AutomaticBindFunctions(fun *dst.FuncDecl) string
	// GenerateExtraFiles used to generate files to setting logger and adapt the tracing context
	GenerateExtraFiles(pkgPath, debugDir string) ([]*rewrite.FileInfo, error)
	// CustomizedEnhance used to customized enhance the log framework
	CustomizedEnhance(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) (map[string]string, bool)
}
