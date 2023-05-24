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

package rewrite

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"

	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

var (
	GlobalOperatorRealSetMethodName = VarPrefix + "OperatorSetOperator"
	GlobalOperatorRealGetMethodName = VarPrefix + "OperatorGetOperator"

	GlobalOperatorTypeName = TypePrefix + "OperatorOperator"
)

func (c *Context) Func(funcDecl *dst.FuncDecl, cursor *dstutil.Cursor) {
	// only the static method needs rewrite
	if funcDecl.Recv == nil {
		// if the method name is generated, then ignore to enhance(for adapter)
		if !strings.HasPrefix(funcDecl.Name.Name, GenerateMethodPrefix) {
			funcDecl.Name = dst.NewIdent(fmt.Sprintf("%s%s%s", StaticMethodPrefix, c.currentPackageTitle, funcDecl.Name.Name))
		}
	} else if len(funcDecl.Recv.List) == 1 {
		// if contains the receiver, then enhance the receiver type
		field := funcDecl.Recv.List[0]
		var typeName string
		if len(field.Names) > 0 {
			typeName = field.Names[0].Name
			c.rewriteMapping.addVarMapping(typeName, typeName)
		}
		if k, v := c.enhanceTypeNameWhenRewrite(field.Type, field, -1); k != "" {
			c.rewriteMapping.addTypeMapping(k, v)
		}
	}

	// enhance method parameter and return value
	c.enhanceFuncParameter(funcDecl.Type.Params)
	c.enhanceFuncParameter(funcDecl.Type.Results)

	// enhance the method body
	for _, stmt := range funcDecl.Body.List {
		c.enhanceFuncStmt(stmt)
	}
}

// nolint
func (c *Context) enhanceFuncStmt(stmt dst.Stmt) {
	// for the variables created in the sub statement, ex: if, func(), the temporary variable count should be recorded
	subCallTypes := []reflect.Type{
		reflect.TypeOf(&dst.IfStmt{}),
		reflect.TypeOf(&dst.BlockStmt{}),
	}
	dstutil.Apply(stmt, func(cursor *dstutil.Cursor) bool {
		for _, t := range subCallTypes {
			if reflect.TypeOf(cursor.Node()) == t {
				c.rewriteMapping.pushBlockStack()
			}
		}
		switch n := cursor.Node().(type) {
		case *dst.BlockStmt:
			for _, tmp := range n.List {
				c.enhanceFuncStmt(tmp)
			}
		case *dst.AssignStmt:
			for _, l := range n.Lhs {
				if k, v := c.enhanceVarNameWhenRewrite(l); k != "" {
					c.rewriteMapping.addVarMapping(k, v)
				}
			}
			for i, r := range n.Rhs {
				if k, v := c.enhanceTypeNameWhenRewrite(r, nil, i); k != "" {
					c.rewriteMapping.addTypeMapping(k, v)
				}
			}
		case *dst.BinaryExpr:
			c.rewriteVarIfExistingMapping(n.X, n)
			c.rewriteVarIfExistingMapping(n.Y, n)
		case *dst.CallExpr:
			c.enhanceTypeNameWhenRewrite(n.Fun, n, -1)
			for inx, arg := range n.Args {
				c.enhanceTypeNameWhenRewrite(arg, n, inx)
			}
		case *dst.ReturnStmt:
			for inx, arg := range n.Results {
				c.enhanceTypeNameWhenRewrite(arg, n, inx)
			}
		case *dst.FuncType:
			c.enhanceFuncParameter(n.Params)
			c.enhanceFuncParameter(n.Results)
		case *dst.ExprStmt:
			c.enhanceTypeNameWhenRewrite(n.X, n, -1)
		case *dst.TypeAssertExpr:
			c.enhanceTypeNameWhenRewrite(n.X, n, -1)
			c.enhanceTypeNameWhenRewrite(n.Type, n, -1)
		case *dst.IfStmt:
			c.enhanceFuncStmt(n.Init)
			c.enhanceTypeNameWhenRewrite(n.Cond, n, -1)
			if n.Body != nil {
				for _, stmt := range n.Body.List {
					c.enhanceFuncStmt(stmt)
				}
			}
			if n.Else != nil {
				c.enhanceFuncStmt(n.Else)
			}
		case *dst.RangeStmt:
			c.enhanceTypeNameWhenRewrite(n.X, n, -1)
			if k, v := c.enhanceVarNameWhenRewrite(n.Key); k != "" {
				c.rewriteMapping.addVarMapping(k, v)
			}
			if k, v := c.enhanceVarNameWhenRewrite(n.Value); k != "" {
				c.rewriteMapping.addVarMapping(k, v)
			}
			if n.Body != nil {
				for _, stmt := range n.Body.List {
					c.enhanceFuncStmt(stmt)
				}
			}
		case *dst.ValueSpec:
			c.Var(n, false)
		default:
			return true
		}

		return false
	}, func(cursor *dstutil.Cursor) bool {
		// all templates variables should be removed
		for _, t := range subCallTypes {
			if reflect.TypeOf(cursor.Node()) == t {
				c.rewriteMapping.popBlockStack()
				break
			}
		}
		return true
	})
}

func (c *Context) rewriteVarIfExistingMapping(exp, parent dst.Expr) bool {
	switch n := exp.(type) {
	case *dst.Ident:
		if v := c.rewriteMapping.findVarMappingName(n.Name); v != "" {
			n.Name = v
			return true
		}
	case *dst.SelectorExpr:
		if pkg, ok := n.X.(*dst.Ident); ok {
			if imp := c.packageImport[pkg.Name]; imp != nil {
				tools.RemovePackageRef(parent, n)
				return true
			}
		}
		return c.rewriteVarIfExistingMapping(n.X, n)
	case *dst.CompositeLit:
		c.enhanceTypeNameWhenRewrite(n.Type, n, -1)
		for _, elt := range n.Elts {
			// for struct data, ex: "&xxx{k: v}"
			if kv, ok := elt.(*dst.KeyValueExpr); ok {
				c.rewriteVarIfExistingMapping(kv.Value, elt)
			}
		}
	case *dst.UnaryExpr:
		c.enhanceTypeNameWhenRewrite(n.X, n, -1)
	case *dst.IndexExpr:
		c.rewriteVarIfExistingMapping(n.Index, n)
		c.rewriteVarIfExistingMapping(n.X, n)
	case *dst.CallExpr:
		c.enhanceTypeNameWhenRewrite(n.Fun, n, -1)
		for _, arg := range n.Args {
			c.rewriteVarIfExistingMapping(arg, n)
		}
	case *dst.StarExpr:
		c.enhanceTypeNameWhenRewrite(n.X, n, -1)
	}
	return false
}

func (c *Context) enhanceFuncParameter(fields *dst.FieldList) {
	if fields == nil {
		return
	}

	for _, field := range fields.List {
		if len(field.Names) > 0 {
			for inx := range field.Names {
				name := field.Names[inx].Name
				// keep the var names for debugging
				c.rewriteMapping.addVarMapping(name, name)
			}
		}
		if k, v := c.enhanceTypeNameWhenRewrite(field.Type, field, -1); k != "" {
			c.rewriteMapping.addTypeMapping(k, v)
		}
	}
}
