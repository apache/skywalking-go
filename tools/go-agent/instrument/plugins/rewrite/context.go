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
	"go/parser"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

var GenerateMethodPrefix = "_skywalking_enhance_"
var GenerateVarPrefix = "_skywalking_var_"
var OperatorDirs = []string{"operator", "log", "tracing"}

var OperatePrefix = "skywalkingOperator"
var TypePrefix = OperatePrefix + "Type"
var VarPrefix = OperatePrefix + "Var"
var StaticMethodPrefix = OperatePrefix + "StaticMethod"

type Context struct {
	pkgFullPath   string
	titleCase     cases.Caser
	targetPackage string

	currentPackageTitle string

	packageImport  map[string]*rewriteImportInfo
	rewriteMapping *rewriteMapping
}

func NewContext(compilePkgFullPath, targetPackage string) *Context {
	c := &Context{
		pkgFullPath:    compilePkgFullPath,
		titleCase:      cases.Title(language.English),
		targetPackage:  targetPackage,
		packageImport:  make(map[string]*rewriteImportInfo),
		rewriteMapping: newRewriteFuncMapping(make(map[string]string), make(map[string]string)),
	}
	// adding self package
	c.packageImport[targetPackage] = &rewriteImportInfo{
		pkgName:     targetPackage,
		isAgentCore: false,
		ctx:         c,
	}
	return c
}

type rewriteImportInfo struct {
	pkgName     string
	isAgentCore bool
	ctx         *Context
}

func (c *Context) IncludeNativeFiles(content string) error {
	parseFile, err := decorator.ParseFile(nil, "native.go", content, parser.ParseComments)
	if err != nil {
		return err
	}

	dstutil.Apply(parseFile, func(cursor *dstutil.Cursor) bool {
		if tp, ok := cursor.Node().(*dst.TypeSpec); ok {
			c.rewriteMapping.addTypeMapping(tp.Name.Name, tp.Name.Name)
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
	return nil
}

func (c *Context) enhanceVarNameWhenRewrite(fieldType dst.Expr) (oldName, replacedName string) {
	switch t := fieldType.(type) {
	case *dst.Ident:
		name := t.Name
		if c.typeIsBasicTypeValueOrEnhanceName(name) {
			return "", ""
		}
		if mappingName := c.rewriteMapping.findVarMappingName(name); mappingName != "" {
			t.Name = mappingName
			return "", ""
		}
		// keep the var name help debugging
		c.rewriteMapping.addVarMapping(name, name)
		return name, t.Name
	case *dst.SelectorExpr:
		return c.enhanceVarNameWhenRewrite(t.X)
	case *dst.IndexExpr:
		c.rewriteVarIfExistingMapping(t.Index, t)
		return c.enhanceVarNameWhenRewrite(t.X)
	}
	return "", ""
}

// nolint
func (c *Context) enhanceTypeNameWhenRewrite(fieldType dst.Expr, parent dst.Node, argIndex int) (string, string) {
	switch t := fieldType.(type) {
	case *dst.Ident:
		name := t.Name
		if c.typeIsBasicTypeValueOrEnhanceName(name) {
			return "", ""
		}
		if c.callIsBasicNamesOrEnhanceName(name) {
			return "", ""
		}
		if mappingName := c.rewriteMapping.findVarMappingName(name); mappingName != "" {
			t.Name = mappingName
			return "", ""
		}
		if mappingName := c.rewriteMapping.findTypeMappingName(name); mappingName != "" {
			t.Name = mappingName
			return "", ""
		}
		// if parent is function call, then the name should be method name
		if _, ok := parent.(*dst.CallExpr); ok {
			t.Name = fmt.Sprintf("%s%s%s", StaticMethodPrefix, c.currentPackageTitle, name)
		} else {
			t.Name = fmt.Sprintf("%s%s%s", TypePrefix, c.currentPackageTitle, name)
		}
		return name, t.Name
	case *dst.SelectorExpr:
		pkgRefName, ok := t.X.(*dst.Ident)
		if !ok {
			return c.enhanceTypeNameWhenRewrite(t.X, parent, -1)
		}
		// reference by package name
		for refImportName, pkgInfo := range c.packageImport {
			if pkgRefName.Name == refImportName {
				switch p := parent.(type) {
				case *dst.CallExpr:
					if c.rewriteVarIfExistingMapping(t.Sel, p) {
						if argIndex >= 0 {
							p.Args[argIndex] = t.Sel
						} else {
							p.Fun = dst.NewIdent(t.Sel.Name)
						}
					} else {
						p.Fun = pkgInfo.generateStaticMethod(t.Sel.Name)
					}
				case *dst.Field:
					p.Type = pkgInfo.generateType(t.Sel.Name)
				case *dst.Ellipsis:
					p.Elt = pkgInfo.generateType(t.Sel.Name)
				case *dst.StarExpr:
					p.X = pkgInfo.generateType(t.Sel.Name)
				case *dst.TypeAssertExpr:
					p.Type = pkgInfo.generateType(t.Sel.Name)
				case *dst.CompositeLit:
					p.Type = pkgInfo.generateType(t.Sel.Name)
				case *dst.ArrayType:
					p.Elt = pkgInfo.generateType(t.Sel.Name)
				}
			}
		}
		// if the method call
		if v := c.rewriteMapping.findVarMappingName(pkgRefName.Name); v != "" {
			t.X = dst.NewIdent(v)
		}
	case *dst.StarExpr:
		return c.enhanceTypeNameWhenRewrite(t.X, t, -1)
	case *dst.ArrayType:
		return c.enhanceTypeNameWhenRewrite(t.Elt, t, -1)
	case *dst.Ellipsis:
		return c.enhanceTypeNameWhenRewrite(t.Elt, t, -1)
	case *dst.CompositeLit:
		for _, elt := range t.Elts {
			// for struct data, ex: "&xxx{k: v}"
			if kv, ok := elt.(*dst.KeyValueExpr); ok {
				c.rewriteVarIfExistingMapping(kv.Value, elt)
			}
		}
		return c.enhanceTypeNameWhenRewrite(t.Type, t, -1)
	case *dst.UnaryExpr:
		return c.enhanceTypeNameWhenRewrite(t.X, t, -1)
	case *dst.CallExpr:
		for inx, arg := range t.Args {
			c.enhanceTypeNameWhenRewrite(arg, t, inx)
		}

		if id, ok := t.Fun.(*dst.Ident); ok {
			if c.callIsBasicNamesOrEnhanceName(id.Name) {
				return "", ""
			}
			return c.enhanceTypeNameWhenRewrite(t.Fun, t, -1)
		}
		c.enhanceTypeNameWhenRewrite(t.Fun, t, -1)
	case *dst.FuncType:
		c.enhanceFuncParameter(t.TypeParams)
		c.enhanceFuncParameter(t.Params)
		c.enhanceFuncParameter(t.Results)
	case *dst.IndexExpr:
		c.rewriteVarIfExistingMapping(t.Index, t)
		return c.enhanceTypeNameWhenRewrite(t.X, t, -1)
	case *dst.TypeAssertExpr:
		c.enhanceTypeNameWhenRewrite(t.Type, t, -1)
		return c.enhanceTypeNameWhenRewrite(t.X, t, -1)
	case *dst.FuncLit:
		c.enhanceTypeNameWhenRewrite(t.Type, t, -1)
		for _, stmt := range t.Body.List {
			c.enhanceFuncStmt(stmt)
		}
	case *dst.BinaryExpr:
		c.enhanceTypeNameWhenRewrite(t.X, t, -1)
		c.enhanceTypeNameWhenRewrite(t.Y, t, -1)
	case *dst.ParenExpr:
		c.rewriteVarIfExistingMapping(t.X, t)
	case *dst.MapType:
		c.enhanceTypeNameWhenRewrite(t.Key, t, -1)
		c.enhanceTypeNameWhenRewrite(t.Value, t, -1)
	}

	return "", ""
}

func (c *Context) typeIsBasicTypeValueOrEnhanceName(name string) bool {
	if strings.HasPrefix(name, OperatePrefix) || strings.HasPrefix(name, GenerateMethodPrefix) || tools.IsBasicDataType(name) ||
		name == "nil" || name == "true" || name == "false" || name == "append" || name == "panic" {
		return true
	}
	if _, valErr := strconv.ParseFloat(name, 64); valErr == nil {
		return true
	}
	return false
}

func (c *Context) callIsBasicNamesOrEnhanceName(name string) bool {
	return strings.HasPrefix(name, OperatePrefix) || strings.HasPrefix(name, GenerateMethodPrefix) ||
		name == "make" || name == "recover" || name == "len"
}

func (r *rewriteImportInfo) generateStaticMethod(name string) *dst.Ident {
	if r.ctx.typeIsBasicTypeValueOrEnhanceName(name) {
		return dst.NewIdent(name)
	}
	if r.isAgentCore {
		return dst.NewIdent(fmt.Sprintf("%s%s%s", StaticMethodPrefix, r.ctx.titleCase.String(r.pkgName), name))
	}
	return dst.NewIdent(name)
}

func (r *rewriteImportInfo) generateType(name string) *dst.Ident {
	if r.isAgentCore {
		return dst.NewIdent(fmt.Sprintf("%s%s%s", TypePrefix, r.ctx.titleCase.String(r.pkgName), name))
	}
	return dst.NewIdent(name)
}

type rewriteMapping struct {
	// push or pop the names when the block statement is called
	rewriteVarNames  []map[string]string
	rewriteTypeNames []map[string]string
}

func newRewriteFuncMapping(varNames, typeNames map[string]string) *rewriteMapping {
	return &rewriteMapping{
		rewriteVarNames:  []map[string]string{varNames},
		rewriteTypeNames: []map[string]string{typeNames},
	}
}

func (m *rewriteMapping) addVarMapping(key, value string) {
	m.rewriteVarNames[len(m.rewriteVarNames)-1][key] = value
}

func (m *rewriteMapping) addTypeMapping(key, value string) {
	m.rewriteTypeNames[len(m.rewriteTypeNames)-1][key] = value
}

func (m *rewriteMapping) pushBlockStack() {
	m.rewriteVarNames = append(m.rewriteVarNames, make(map[string]string))
	m.rewriteTypeNames = append(m.rewriteTypeNames, make(map[string]string))
}

func (m *rewriteMapping) popBlockStack() {
	m.rewriteVarNames = m.rewriteVarNames[:len(m.rewriteVarNames)-1]
	m.rewriteTypeNames = m.rewriteTypeNames[:len(m.rewriteTypeNames)-1]
}

func (m *rewriteMapping) findVarMappingName(name string) string {
	for i := len(m.rewriteVarNames) - 1; i >= 0; i-- {
		if v, ok := m.rewriteVarNames[i][name]; ok {
			return v
		}
	}
	return ""
}

func (m *rewriteMapping) findTypeMappingName(name string) string {
	for i := len(m.rewriteTypeNames) - 1; i >= 0; i-- {
		if v, ok := m.rewriteTypeNames[i][name]; ok {
			return v
		}
	}
	return ""
}
