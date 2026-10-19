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
	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

var defaultInternalAtomicPath = "runtime/internal/atomic"

type Instrument struct {
	goIDType string
	opts     *api.CompileOptions
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (r *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	r.opts = opts
	return opts.Package == "runtime"
}

func (r *Instrument) FilterAndEdit(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	switch n := cursor.Node().(type) {
	case *dst.TypeSpec:
		if n.Name != nil && n.Name.Name != "g" {
			return false
		}
		st, ok := n.Type.(*dst.StructType)
		if !ok {
			return false
		}
		for _, f := range st.Fields.List {
			if len(f.Names) > 0 && f.Names[0].Name == "goid" {
				r.goIDType = f.Type.(*dst.Ident).Name
			}
		}
		// append the tls field, and the field that passes the context snapshot of a
		// goroutine to the goroutine it creates
		st.Fields.List = append(st.Fields.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(consts.TLSFieldName)},
			Type:  dst.NewIdent("interface{}")}, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(consts.TLSSnapshotFieldName)},
			Type:  dst.NewIdent("interface{}")})
		tools.LogWithStructEnhance("runtime", "g", consts.TLSFieldName, "tls field")
		return true
	case *dst.FuncDecl:
		if n.Recv != nil {
			return false
		}
		switch n.Name.Name {
		case "newproc", "newcoro":
			return r.enhanceGoroutineCreator(n)
		case "newproc1":
			return r.enhanceNewProc1(n)
		}
	}
	return false
}

// enhanceGoroutineCreator takes the context snapshot for the new goroutine in
// the functions that call newproc1. They run on the stack of the goroutine
// that creates the new one, before they switch to the system stack, so the
// snapshot can take the span and context locks. newproc1 runs on the system
// stack, where blocking on a contended lock is a fatal error.
//
// The snapshot is taken right before the switch, so that no allocation (which
// can start GC workers through a nested newproc) runs between taking and
// handing it over. It is only taken on a user goroutine that holds no runtime
// locks: the runtime starts its GC workers between acquirem and releasem, where
// blocking is a fatal error too, and they need no trace context.
func (r *Instrument) enhanceGoroutineCreator(n *dst.FuncDecl) bool {
	if len(n.Type.Params.List) != 1 {
		return false
	}
	switchIndex := -1
	for i, stmt := range n.Body.List {
		if isSystemStackCall(stmt) {
			switchIndex = i
		}
	}
	if switchIndex < 0 {
		return false
	}
	stmts := tools.GoStringToStats(tools.ExecuteTemplate(`if gp := getg(); gp == gp.m.curg && gp.m.locks == 0 && gp.m.preemptoff == "" {
	gp.{{.SnapshotField}} = goroutineChange(gp.{{.TLSField}})
}
`, struct {
		TLSField      string
		SnapshotField string
	}{
		TLSField:      consts.TLSFieldName,
		SnapshotField: consts.TLSSnapshotFieldName,
	}))
	body := make([]dst.Stmt, 0, len(n.Body.List)+len(stmts))
	body = append(body, n.Body.List[:switchIndex]...)
	body = append(body, stmts...)
	n.Body.List = append(body, n.Body.List[switchIndex:]...)
	tools.LogWithMethodEnhance("runtime", "", n.Name.Name, "take the context snapshot for the new goroutine")
	return true
}

// isSystemStackCall reports whether the statement is a systemstack(...) call.
func isSystemStackCall(stmt dst.Stmt) bool {
	expr, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expr.X.(*dst.CallExpr)
	if !ok {
		return false
	}
	fun, ok := call.Fun.(*dst.Ident)
	return ok && fun.Name == "systemstack"
}

// enhanceNewProc1 hands the snapshot taken by the caller to the new goroutine.
// It only moves the reference, because newproc1 runs on the system stack.
func (r *Instrument) enhanceNewProc1(n *dst.FuncDecl) bool {
	if n.Type.Results == nil || len(n.Type.Results.List) != 1 {
		return false
	}
	expectedParamCount := 3
	if r.opts.CheckGoVersionGreaterOrEqual(1, 23) {
		expectedParamCount = 5
	}
	if len(n.Type.Params.List) != expectedParamCount {
		return false
	}
	parameters := tools.EnhanceParameterNames(n.Type.Params, tools.FieldListTypeParam)
	results := tools.EnhanceParameterNames(n.Type.Results, tools.FieldListTypeResult)

	tools.InsertStmtsBeforeBody(n.Body, `defer func() {
	{{(index .Results 0).Name}}.{{.TLSField}} = {{(index .Parameters 1).Name}}.{{.SnapshotField}}
	{{(index .Parameters 1).Name}}.{{.SnapshotField}} = nil
}()
`, struct {
		Parameters    []*tools.ParameterInfo
		Results       []*tools.ParameterInfo
		TLSField      string
		SnapshotField string
	}{
		Parameters:    parameters,
		Results:       results,
		TLSField:      consts.TLSFieldName,
		SnapshotField: consts.TLSSnapshotFieldName,
	})
	tools.LogWithMethodEnhance("runtime", "", "newproc1", "support cross goroutine context propagating")
	return true
}

func (r *Instrument) AfterEnhanceFile(fromPath, newPath string) error {
	return nil
}

func (r *Instrument) parseInternalAtomicPath() string {
	if r.opts.CheckGoVersionGreaterOrEqual(1, 23) {
		return "internal/runtime/atomic"
	}
	return defaultInternalAtomicPath
}

// nolint
func (r *Instrument) WriteExtraFiles(dir string) ([]string, error) {
	return tools.WriteMultipleFile(dir, map[string]string{
		"skywalking_tls_operator.go": tools.ExecuteTemplate(`package runtime

import (
	_ "unsafe"

	atomic "{{.InternalAtomicPath}}"
)

var {{.GlobalTracerFieldName}} interface{}

var {{.GlobalLoggerFieldName}} interface{}

var {{.GlobalTracerInitNotifyFieldName}} = make([]func(), 0)

var _metricsRegisterLockerVal int32 = 0
var _metricsRegisterLocker = &_metricsRegisterLockerVal
var {{.MetricsRegisterFieldName}} = make([]interface{}, 0)
var {{.MetricsHookFieldName}} = make([]func(), 0)

//go:linkname {{.TLSGetMethod}} {{.TLSGetMethod}}
var {{.TLSGetMethod}} = _skywalking_tls_get_impl

//go:linkname {{.TLSSetMethod}} {{.TLSSetMethod}}
var {{.TLSSetMethod}} = _skywalking_tls_set_impl

//go:linkname {{.GlobalOperatorSetMethodName}} {{.GlobalOperatorSetMethodName}}
var {{.GlobalOperatorSetMethodName}} = _skywalking_global_operator_set_impl

//go:linkname {{.GlobalOperatorGetMethodName}} {{.GlobalOperatorGetMethodName}}
var {{.GlobalOperatorGetMethodName}} = _skywalking_global_operator_get_impl

//go:linkname {{.GlobalLoggerSetMethodName}} {{.GlobalLoggerSetMethodName}}
var {{.GlobalLoggerSetMethodName}} = _skywalking_global_logger_set_impl

//go:linkname {{.GlobalLoggerGetMethodName}} {{.GlobalLoggerGetMethodName}}
var {{.GlobalLoggerGetMethodName}} = _skywalking_global_logger_get_impl

//go:linkname {{.GoroutineIDGetterMethodName}} {{.GoroutineIDGetterMethodName}}
var {{.GoroutineIDGetterMethodName}} = _skywalking_get_goid_impl

//go:linkname {{.GlobalTracerInitNotifyMethodName}} {{.GlobalTracerInitNotifyMethodName}}
var {{.GlobalTracerInitNotifyMethodName}} = _skywalking_global_tracer_init_notify_impl

//go:linkname {{.GlobalTracerInitNotifyGetMethodName}} {{.GlobalTracerInitNotifyGetMethodName}}
var {{.GlobalTracerInitNotifyGetMethodName}} = _skywalking_global_tracer_init_get_notify_impl

//go:linkname {{.MetricsRegisterAppendMethodName}} {{.MetricsRegisterAppendMethodName}}
var {{.MetricsRegisterAppendMethodName}} = _skywalking_metrics_register_append_impl

//go:linkname {{.MetricsObtainMethodName}} {{.MetricsObtainMethodName}}
var {{.MetricsObtainMethodName}} = _skywalking_metrics_obtain_impl

//go:nosplit
func _skywalking_get_goid_impl() int64 {
	return {{.GoroutineIDCaster}}
}

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
	{{.GlobalTracerFieldName}} = v
} 

//go:nosplit
func _skywalking_global_operator_get_impl() interface{} {
	return {{.GlobalTracerFieldName}}
} 

//go:nosplit
func _skywalking_global_logger_set_impl(v interface{}) {
	{{.GlobalLoggerFieldName}} = v
} 

//go:nosplit
func _skywalking_global_logger_get_impl() interface{} {
	return {{.GlobalLoggerFieldName}}
} 

//go:nosplit
func _skywalking_global_tracer_init_notify_impl(fun func()) {
	{{.GlobalTracerInitNotifyFieldName}} = append({{.GlobalTracerInitNotifyFieldName}}, fun)
}

//go:nosplit
func _skywalking_global_tracer_init_get_notify_impl() []func() {
	return {{.GlobalTracerInitNotifyFieldName}}
}

//go:nosplit
func _skywalking_metrics_register_append_impl(v interface{}) {
	for {
		tmp := atomic.Loadint32(_metricsRegisterLocker)
		if atomic.Casint32(_metricsRegisterLocker, tmp, tmp+1) {
			{{.MetricsRegisterFieldName}} = append({{.MetricsRegisterFieldName}}, v)
			break
		}
	}
}

//go:nosplit
func _skywalking_metrics_obtain_impl() ([]interface{}, []func()) {
	for {
		tmp := atomic.Loadint32(_metricsRegisterLocker)
		if tmp == 0 {
			return nil, nil
		}
		if atomic.Casint32(_metricsRegisterLocker, tmp, 0) {
			registers := {{.MetricsRegisterFieldName}}
			{{.MetricsRegisterFieldName}} = make([]interface{}, 0)
			hooks := {{.MetricsHookFieldName}}
			{{.MetricsHookFieldName}} = make([]func(), 0)
			return registers, hooks
		}
	}
}

//go:nosplit
func _skywalking_metrics_hook_append_impl(f func()) {
	for {
		tmp := atomic.Loadint32(_metricsRegisterLocker)
		if atomic.Casint32(_metricsRegisterLocker, tmp, tmp+1) {
			{{.MetricsHookFieldName}} = append({{.MetricsHookFieldName}}, f)
			break
		}
	}
}

type ContextSnapshoter interface {
	TakeSnapShot() interface{}
}

func goroutineChange(tls interface{}) interface{} {
	if tls == nil {
		return nil
	}
	if taker, ok := tls.(ContextSnapshoter); ok && taker != nil {
		return taker.TakeSnapShot()
	}
	return tls
}
`, struct {
			TLSFiledName                        string
			TLSGetMethod                        string
			TLSSetMethod                        string
			GlobalTracerFieldName               string
			GlobalTracerSnapshotInterface       string
			GlobalOperatorSetMethodName         string
			GlobalOperatorGetMethodName         string
			GlobalLoggerFieldName               string
			GlobalLoggerSetMethodName           string
			GlobalLoggerGetMethodName           string
			GoroutineIDGetterMethodName         string
			GoroutineIDCaster                   string
			GlobalTracerInitNotifyFieldName     string
			GlobalTracerInitNotifyMethodName    string
			GlobalTracerInitNotifyGetMethodName string
			MetricsRegisterFieldName            string
			MetricsRegisterAppendMethodName     string
			MetricsObtainMethodName             string
			MetricsHookFieldName                string
			MetricsHookAppendMethodName         string
			InternalAtomicPath                  string
		}{
			TLSFiledName:                        consts.TLSFieldName,
			TLSGetMethod:                        consts.TLSGetMethodName,
			TLSSetMethod:                        consts.TLSSetMethodName,
			GlobalTracerFieldName:               consts.GlobalTracerFieldName,
			GlobalTracerSnapshotInterface:       consts.GlobalTracerSnapshotInterface,
			GlobalOperatorSetMethodName:         consts.GlobalTracerSetMethodName,
			GlobalOperatorGetMethodName:         consts.GlobalTracerGetMethodName,
			GlobalLoggerFieldName:               consts.GlobalLoggerFieldName,
			GlobalLoggerSetMethodName:           consts.GlobalLoggerSetMethodName,
			GlobalLoggerGetMethodName:           consts.GlobalLoggerGetMethodName,
			GoroutineIDGetterMethodName:         consts.CurrentGoroutineIDGetMethodName,
			GoroutineIDCaster:                   r.generateCastGoID("getg().m.curg.goid"),
			GlobalTracerInitNotifyFieldName:     consts.GlobalTracerInitNotifyFieldName,
			GlobalTracerInitNotifyMethodName:    consts.GlobalTracerInitAppendNotifyMethodName,
			GlobalTracerInitNotifyGetMethodName: consts.GlobalTracerInitGetNotifyMethodName,
			MetricsRegisterFieldName:            consts.MetricsRegisterFieldName,
			MetricsRegisterAppendMethodName:     consts.MetricsRegisterAppendMethodName,
			MetricsObtainMethodName:             consts.MetricsObtainMethodName,
			MetricsHookFieldName:                consts.MetricsHookFieldName,
			MetricsHookAppendMethodName:         consts.MetricsHookAppendMethodName,
			InternalAtomicPath:                  r.parseInternalAtomicPath(),
		}),
	})
}

func (r *Instrument) generateCastGoID(val string) string {
	switch r.goIDType {
	case "int64":
		return val
	case "uint64":
	case "int32":
	case "uint32":
	case "int":
	case "uint":
	default:
		panic("cannot find goid type in the g struct or the type is not supported: " + r.goIDType)
	}
	return "int64(" + val + ")"
}
