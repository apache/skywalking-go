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

package runtime

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

var (
	TLSFieldName     = "skywalking_tls"
	TLSGetMethodName = "_skywalking_get_gls"
	TLSSetMethodName = "_skywalking_set_gls"

	GlobalTracerFieldName         = "globalSkyWalkingOperator"
	GlobalTracerSnapshotInterface = "skywalkingGoroutineSnapshotCreator"
	GlobalTracerSetMethodName     = "_skywalking_set_global_operator"
	GlobalTracerGetMethodName     = "_skywalking_get_global_operator"
)

type Instrument struct {
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (r *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	return opts.Package == "runtime"
}

func (r *Instrument) FilterAndEdit(path string, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	switch n := cursor.Node().(type) {
	case *dst.TypeSpec:
		if n.Name != nil && n.Name.Name != "g" {
			return false
		}
		st, ok := n.Type.(*dst.StructType)
		if !ok {
			return false
		}
		// append the tls field
		st.Fields.List = append(st.Fields.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(TLSFieldName)},
			Type:  dst.NewIdent("interface{}")})
		tools.LogWithStructEnhance("runtime", "g", TLSFieldName, "tls field")
		return true
	case *dst.FuncDecl:
		if n.Name.Name != "newproc1" {
			return false
		}
		if len(n.Type.Results.List) != 1 {
			return false
		}
		if len(n.Type.Params.List) != 3 {
			return false
		}
		parameters := tools.EnhanceParameterNames(n.Type.Params, false)
		results := tools.EnhanceParameterNames(n.Type.Results, true)

		tools.InsertStmtsBeforeBody(n.Body, `defer func() {
	{{(index .Results 0).Name}}.{{.TLSField}} = goroutineChange({{(index .Parameters 1).Name}}.{{.TLSField}})
}()
`, struct {
			Parameters        []*tools.ParameterInfo
			Results           []*tools.ParameterInfo
			TLSField          string
			OperatorField     string
			SnapshotInterface string
		}{
			Parameters:        parameters,
			Results:           results,
			TLSField:          TLSFieldName,
			OperatorField:     GlobalTracerFieldName,
			SnapshotInterface: GlobalTracerSnapshotInterface,
		})
		tools.LogWithMethodEnhance("runtime", "", "newproc1", "support cross goroutine context propagating")
		return true
	}
	return false
}

func (r *Instrument) AfterEnhanceFile(fromPath, newPath string) error {
	return nil
}

func (r *Instrument) WriteExtraFiles(dir string) ([]string, error) {
	return tools.WriteMultipleFile(dir, map[string]string{
		"skywalking_tls_operator.go": tools.ExecuteTemplate(`package runtime

import (
	_ "unsafe"
)

var {{.GlobalTracerFieldName}} interface{}

//go:linkname {{.TLSGetMethod}} {{.TLSGetMethod}}
var {{.TLSGetMethod}} = _skywalking_tls_get_impl

//go:linkname {{.TLSSetMethod}} {{.TLSSetMethod}}
var {{.TLSSetMethod}} = _skywalking_tls_set_impl

//go:linkname {{.GlobalOperatorSetMethodName}} {{.GlobalOperatorSetMethodName}}
var {{.GlobalOperatorSetMethodName}} = _skywalking_global_operator_set_impl

//go:linkname {{.GlobalOperatorGetMethodName}} {{.GlobalOperatorGetMethodName}}
var {{.GlobalOperatorGetMethodName}} = _skywalking_global_operator_get_impl

//go:nosplit
func _skywalking_tls_get_impl() interface{} {
	return getg().m.curg.{{.TLSFiledName}}
}

//go:nosplit
func _skywalking_tls_set_impl(v interface{}) {
	getg().m.curg.{{.TLSFiledName}} = v
}

//go:nosplit
func _skywalking_global_operator_set_impl(v interface{}) {
	globalSkyWalkingOperator = v
} 

//go:nosplit
func _skywalking_global_operator_get_impl() interface{} {
	return globalSkyWalkingOperator
} 

type ContextSnapshoter interface {
	TakeSnapShot(val interface{}) interface{}
}

func goroutineChange(tls interface{}) interface{} {
	if tls == nil {
		return nil
	}
	if taker, ok := tls.(ContextSnapshoter); ok {
		return taker.TakeSnapShot(tls)
	}
	return tls
}
`, struct {
			TLSFiledName                  string
			TLSGetMethod                  string
			TLSSetMethod                  string
			GlobalTracerFieldName         string
			GlobalTracerSnapshotInterface string
			GlobalOperatorSetMethodName   string
			GlobalOperatorGetMethodName   string
		}{
			TLSFiledName:                  TLSFieldName,
			TLSGetMethod:                  TLSGetMethodName,
			TLSSetMethod:                  TLSSetMethodName,
			GlobalTracerFieldName:         GlobalTracerFieldName,
			GlobalTracerSnapshotInterface: GlobalTracerSnapshotInterface,
			GlobalOperatorSetMethodName:   GlobalTracerSetMethodName,
			GlobalOperatorGetMethodName:   GlobalTracerGetMethodName,
		}),
	})
}
