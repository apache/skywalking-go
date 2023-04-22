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

	"github.com/dave/dst"
)

func (c *Context) Type(tp *dst.TypeSpec) {
	oldName := tp.Name.Name
	tp.Name = dst.NewIdent(fmt.Sprintf("%s%s%s", TypePrefix, c.currentPackageTitle, oldName))
	c.rewriteMapping.addTypeMapping(oldName, tp.Name.Name)

	// define interface type, ex: "type xxx interface {}"
	if inter, ok := tp.Type.(*dst.InterfaceType); ok {
		for _, method := range inter.Methods.List {
			funcType, ok := method.Type.(*dst.FuncType)
			if !ok {
				continue
			}

			c.enhanceFuncParameter(funcType.Params)
			c.enhanceFuncParameter(funcType.Results)
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
