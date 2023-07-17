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

	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
)

func (c *Context) Type(tp *dst.TypeSpec, parent dst.Node, onlyName bool) {
	oldName := tp.Name.Name
	if !c.alreadyGenerated(oldName) {
		tp.Name = dst.NewIdent(fmt.Sprintf("%s%s%s", c.generateTypePrefix(parent), c.currentPackageTitle, oldName))
		c.rewriteMapping.addTypeMapping(oldName, tp.Name.Name)
	}
	if onlyName {
		return
	}

	// define interface type, ex: "type xxx interface {}"
	if inter, ok := tp.Type.(*dst.InterfaceType); ok {
		for _, method := range inter.Methods.List {
			switch t := method.Type.(type) {
			case *dst.Ident:
				c.enhanceTypeNameWhenRewrite(t, tp.Type, -1)
			case *dst.FuncType:
				c.enhanceFuncParameter(t.Params)
				c.enhanceFuncParameter(t.Results)
			}
		}
	}

	// define func type, ex: "type xxx func(x X) x"
	if funcType, ok := tp.Type.(*dst.FuncType); ok {
		c.enhanceFuncParameter(funcType.Params)
		c.enhanceFuncParameter(funcType.Results)
	}

	// define struct type, ex: "type xx struct {}"
	if structType, ok := tp.Type.(*dst.StructType); ok && structType.Fields != nil {
		for _, field := range structType.Fields.List {
			c.enhanceTypeNameWhenRewrite(field.Type, field, -1)
		}
	}
}

func (c *Context) generateTypePrefix(parent dst.Node) string {
	prefix := TypePrefix
	if parent == nil || !tools.ContainsDirective(parent, consts.DirectivePublic) {
		return prefix
	}
	return c.titleCase.String(TypePrefix)
}
