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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

var GenerateCommonPrefix = "skywalking_"

var GenerateMethodPrefix = GenerateCommonPrefix + "enhance_"
var GenerateVarPrefix = GenerateCommonPrefix + "var_"
var OperatorDirs = []string{"operator", "log", "tracing", "tools", "metrics"}

var OperatePrefix = GenerateCommonPrefix + "operator"
var TypePrefix = OperatePrefix + "Type"
var VarPrefix = OperatePrefix + "Var"
var StaticMethodPrefix = OperatePrefix + "StaticMethod"

type Context struct {
	pkgFullPath   string
	titleCase     cases.Caser
	targetPackage string

	currentPackageTitle string

	currentProcessingFile *dst.File

	packageImport  map[string]*rewriteImportInfo
	rewriteMapping *rewriteMapping

	InitFuncDetector []string
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

func (c *Context) appendInitFunction(name string) {
	c.InitFuncDetector = append(c.InitFuncDetector, name)
}

type rewriteImportInfo struct {
	pkgName     string
	isAgentCore bool
	ctx         *Context
}

func (c *Context) IncludeNativeOrReferenceGenerateFiles(content string) error {
	parseFile, err := decorator.ParseFile(nil, "ref.go", content, parser.ParseComments)
	if err != nil {
		return err
	}

	dstutil.Apply(parseFile, func(cursor *dstutil.Cursor) bool {
		switch n := cursor.Node().(type) {
		case *dst.TypeSpec:
			c.analyzeNativeOrReferenceFields(cursor.Node(), cursor.Parent(), n.Name.Name)
		case *dst.FuncDecl:
			c.analyzeNativeOrReferenceFields(cursor.Node(), cursor.Node(), n.Name.Name)
		}
		return true
	}, func(cursor *dstutil.Cursor) bool {
		return true
	})
	return nil
}

func (c *Context) analyzeNativeOrReferenceFields(node, decorationNode dst.Node, currentDeclareName string) {
	if native := tools.FindDirective(decorationNode, consts.DirectiveNative); native != "" {
		name, typeName := c.analyzeNativeTypeDirective(native)
		c.rewriteMapping.addNativeTypeMapping(currentDeclareName, name, typeName)
	}
	if reference := tools.FindDirective(decorationNode, consts.DirectiveReferenceGenerate); reference != "" {
		name, typeName := c.analyzeReferenceGenerateDirective(reference)
		c.rewriteMapping.addReferenceGenerateMapping(node, currentDeclareName, name, typeName)
	}
}

func (c *Context) analyzeNativeTypeDirective(comment string) (packageName, typeName string) {
	info := strings.SplitN(comment, " ", 3)
	if len(info) != 3 {
		panic(fmt.Sprintf("failure to parse the skywalking:native directive: %s", comment))
	}
	return info[1], info[2]
}

func (c *Context) analyzeReferenceGenerateDirective(comment string) (packageName, typeName string) {
	info := strings.SplitN(comment, " ", 3)
	if len(info) != 3 {
		panic(fmt.Sprintf("failure to parse the skywalking:native directive: %s", comment))
	}
	return info[1], info[2]
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
		if native := c.rewriteMapping.findNativeTypeMapping(name); native != nil {
			// change to the selector expr(package.type) and enhance
			pkgBase := filepath.Base(native.packageName)
			// check the package have been import or not
			c.AddingImportToCurrentFile(pkgBase, native.packageName)
			c.enhanceTypeNameWhenRewrite(&dst.SelectorExpr{
				X:   dst.NewIdent(pkgBase),
				Sel: dst.NewIdent(native.typeName),
			}, parent, argIndex)
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
		var foundPackage *rewriteImportInfo
		for refImportName, pkgInfo := range c.packageImport {
			if pkgRefName.Name == refImportName {
				foundPackage = pkgInfo
				break
			}
		}
		// if the method call
		if v := c.rewriteMapping.findVarMappingName(pkgRefName.Name); v != "" {
			t.X = dst.NewIdent(v)
			return "", ""
		}
		// is the other package reference
		if strings.HasPrefix(pkgRefName.Name, tools.OtherPackageRefPrefix) {
			t.X = dst.NewIdent(pkgRefName.Name[len(tools.OtherPackageRefPrefix):])
			return "", ""
		}

		var generateExpr func() dst.Expr
		var generateCallExpr func(parent *dst.CallExpr)
		if foundPackage != nil {
			generateCallExpr = func(parent *dst.CallExpr) {
				if c.rewriteVarIfExistingMapping(t.Sel, parent) {
					if argIndex >= 0 {
						parent.Args[argIndex] = t.Sel
					} else {
						parent.Fun = dst.NewIdent(t.Sel.Name)
					}
				} else {
					parent.Fun = foundPackage.generateStaticMethod(t.Sel.Name)
				}
			}
			generateExpr = func() dst.Expr {
				return foundPackage.generateType(t.Sel.Name)
			}
		} else {
			// if it cannot found the package, then it just keep the data
			generateCallExpr = func(parent *dst.CallExpr) {
				if argIndex >= 0 {
					parent.Args[argIndex] = fieldType
				} else {
					parent.Fun = fieldType
				}
			}
			generateExpr = func() dst.Expr {
				return fieldType
			}
		}

		switch p := parent.(type) {
		case *dst.CallExpr:
			generateCallExpr(p)
		case *dst.Field:
			p.Type = generateExpr()
		case *dst.Ellipsis:
			p.Elt = generateExpr()
		case *dst.StarExpr:
			p.X = generateExpr()
		case *dst.TypeAssertExpr:
			p.Type = generateExpr()
		case *dst.CompositeLit:
			p.Type = generateExpr()
		case *dst.ArrayType:
			p.Elt = generateExpr()
		case *dst.ValueSpec:
			p.Type = generateExpr()
		case *dst.BinaryExpr:
			if argIndex == 0 {
				p.X = generateExpr()
			} else if argIndex == 1 {
				p.Y = generateExpr()
			} else {
				panic("binary expr arg index error")
			}
		case *dst.CaseClause:
			if argIndex < 0 {
				panic("case clause arg index error")
			}
			p.List[argIndex] = generateExpr()
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
			} else if call, ok := elt.(*dst.CallExpr); ok {
				c.enhanceTypeNameWhenRewrite(call, t, -1)
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
		c.enhanceTypeNameWhenRewrite(t.X, t, 0)
		c.enhanceTypeNameWhenRewrite(t.Y, t, 1)
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
		name == "nil" || name == "true" || name == "false" || name == "append" || name == "panic" || name == "new" {
		return true
	}
	if _, valErr := strconv.ParseFloat(name, 64); valErr == nil {
		return true
	}
	return false
}

func (c *Context) alreadyGenerated(name string) bool {
	return strings.HasPrefix(name, GenerateCommonPrefix) || strings.HasPrefix(name, c.titleCase.String(GenerateCommonPrefix))
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
	nativeTypes      map[string]*nativeType
}

type nativeType struct {
	packageName string
	typeName    string
}

func newRewriteFuncMapping(varNames, typeNames map[string]string) *rewriteMapping {
	return &rewriteMapping{
		rewriteVarNames:  []map[string]string{varNames},
		rewriteTypeNames: []map[string]string{typeNames},
		nativeTypes:      map[string]*nativeType{},
	}
}

func (m *rewriteMapping) addVarMapping(key, value string) {
	m.rewriteVarNames[len(m.rewriteVarNames)-1][key] = value
}

func (m *rewriteMapping) addTypeMapping(key, value string) {
	m.rewriteTypeNames[len(m.rewriteTypeNames)-1][key] = value
}

func (m *rewriteMapping) addNativeTypeMapping(key, packageName, typeName string) {
	m.nativeTypes[key] = &nativeType{
		packageName: packageName,
		typeName:    typeName,
	}
}

func (m *rewriteMapping) addReferenceGenerateMapping(node dst.Node, key, packageName, typeName string) {
	titleCase := cases.Title(language.English)
	packageTitle := titleCase.String(filepath.Base(packageName))
	switch node.(type) {
	case *dst.FuncDecl:
		m.addNativeTypeMapping(key, packageName,
			fmt.Sprintf("%s%s%s", titleCase.String(GenerateMethodPrefix), packageTitle, typeName))
	case *dst.TypeSpec:
		m.addNativeTypeMapping(key, packageName,
			fmt.Sprintf("%s%s%s", titleCase.String(TypePrefix), packageTitle, typeName))
	}
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

func (m *rewriteMapping) findNativeTypeMapping(name string) *nativeType {
	return m.nativeTypes[name]
}
