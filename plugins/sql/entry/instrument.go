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
	"embed"

	"github.com/apache/skywalking-go/plugins/core/instrument"
)

//go:embed *
var fs embed.FS

//skywalking:nocopy
type Instrument struct {
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "sql"
}

func (i *Instrument) BasePackage() string {
	return "database/sql"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

// nolint
func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "",
			At:          instrument.NewStructEnhance("DB"),
		},
		{
			PackagePath: "",
			At:          instrument.NewStructEnhance("Stmt"),
		},
		{
			PackagePath: "",
			At:          instrument.NewStructEnhance("Tx"),
		},
		{
			PackagePath: "",
			At:          instrument.NewStructEnhance("Conn"),
		},
		{
			PackagePath: "",
			At: instrument.NewStaticMethodEnhance("Open",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "string"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*DB"), instrument.WithResultType(1, "error")),
			Interceptor: "InstanceInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*DB", "PingContext",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "context.Context"),
				instrument.WithResultCount(1), instrument.WithResultType(0, "error")),
			Interceptor: "PingInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*DB", "PrepareContext",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Stmt"), instrument.WithResultType(1, "error")),
			Interceptor: "PrepareInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*DB", "ExecContext",
				instrument.WithArgsCount(3), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "Result"), instrument.WithResultType(1, "error")),
			Interceptor: "ExecInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*DB", "QueryContext",
				instrument.WithArgsCount(3), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Rows"), instrument.WithResultType(1, "error")),
			Interceptor: "QueryInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*DB", "BeginTx",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "*TxOptions"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Tx"), instrument.WithResultType(1, "error")),
			Interceptor: "BeginTXInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*DB", "Conn",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "context.Context"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Conn"), instrument.WithResultType(1, "error")),
			Interceptor: "ConnInterceptor",
		},
		// Conn operation
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Conn", "PingContext",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "context.Context"),
				instrument.WithResultCount(1), instrument.WithResultType(0, "error")),
			Interceptor: "ConnPingInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Conn", "ExecContext",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithArgType(0, "Result"), instrument.WithResultType(1, "error")),
			Interceptor: "ConnExecInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Conn", "QueryContext",
				instrument.WithArgsCount(3),
				instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithArgType(0, "*Rows"), instrument.WithResultType(1, "error")),
			Interceptor: "ConnQueryInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Conn", "PrepareContext",
				instrument.WithArgsCount(2),
				instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithArgType(0, "*Stmt"), instrument.WithResultType(1, "error")),
			Interceptor: "ConnPrepareInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Conn", "Raw",
				instrument.WithArgsCount(1),
				instrument.WithResultCount(1), instrument.WithResultType(0, "error")),
			Interceptor: "ConnRawInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Conn", "BeginTx",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "*TxOptions"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Tx"), instrument.WithResultType(1, "error")),
			Interceptor: "ConnBeginTXInterceptor",
		},
		// TX operation
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Tx", "Commit",
				instrument.WithArgsCount(0),
				instrument.WithResultCount(1), instrument.WithResultType(0, "error")),
			Interceptor: "TxCommitInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Tx", "Rollback",
				instrument.WithArgsCount(0),
				instrument.WithResultCount(1), instrument.WithResultType(0, "error")),
			Interceptor: "TxRollbackInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Tx", "PrepareContext",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Stmt"), instrument.WithResultType(1, "error")),
			Interceptor: "TxPrepareInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Tx", "StmtContext",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "*Stmt"),
				instrument.WithResultCount(1), instrument.WithResultType(0, "*Stmt")),
			Interceptor: "TxStmtInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Tx", "ExecContext",
				instrument.WithArgsCount(3), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "Result"), instrument.WithResultType(1, "error")),
			Interceptor: "TxExecInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Tx", "QueryContext",
				instrument.WithArgsCount(3), instrument.WithArgType(0, "context.Context"), instrument.WithArgType(1, "string"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Rows"), instrument.WithResultType(1, "error")),
			Interceptor: "TxQueryInterceptor",
		},
		// Stmt Operation
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Stmt", "ExecContext",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "context.Context"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "Result"), instrument.WithResultType(1, "error")),
			Interceptor: "StmtExecInterceptor",
		},
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("*Stmt", "QueryContext",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "context.Context"),
				instrument.WithResultCount(2), instrument.WithResultType(0, "*Rows"), instrument.WithResultType(1, "error")),
			Interceptor: "StmtQueryInterceptor",
		},
	}
}

func (i *Instrument) PluginSourceCodePath() string {
	return "entry"
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
