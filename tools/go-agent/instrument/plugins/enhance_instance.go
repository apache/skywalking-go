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

package plugins

import (
	"fmt"

	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
)

var EnhanceInstanceField = "skywalkingDynamicField"

type InstanceEnhance struct {
	typeSpec *dst.TypeSpec
}

func NewInstanceEnhance(typeSpec *dst.TypeSpec) *InstanceEnhance {
	return &InstanceEnhance{typeSpec: typeSpec}
}

func (i *InstanceEnhance) EnhanceField() {
	structType := i.typeSpec.Type.(*dst.StructType)
	structType.Fields.List = append(structType.Fields.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(EnhanceInstanceField)},
		Type:  dst.NewIdent("interface{}"),
	})
}

func (i *InstanceEnhance) BuildForAdapter() []dst.Decl {
	return []dst.Decl{
		&dst.FuncDecl{
			Name: &dst.Ident{Name: "GetSkyWalkingDynamicField"},
			Recv: &dst.FieldList{
				List: []*dst.Field{
					{
						Names: []*dst.Ident{dst.NewIdent("receiver")},
						Type:  &dst.StarExpr{X: dst.NewIdent(i.typeSpec.Name.Name)},
					},
				},
			},
			Type: &dst.FuncType{
				Params: &dst.FieldList{},
				Results: &dst.FieldList{
					List: []*dst.Field{
						{Type: dst.NewIdent("interface{}")},
					},
				},
			},
			Body: &dst.BlockStmt{
				List: tools.GoStringToStats(fmt.Sprintf("return receiver.%s", EnhanceInstanceField)),
			},
		},
		&dst.FuncDecl{
			Name: &dst.Ident{Name: "SetSkyWalkingDynamicField"},
			Recv: &dst.FieldList{
				List: []*dst.Field{
					{
						Names: []*dst.Ident{dst.NewIdent("receiver")},
						Type:  &dst.StarExpr{X: dst.NewIdent(i.typeSpec.Name.Name)},
					},
				},
			},
			Type: &dst.FuncType{
				Params: &dst.FieldList{
					List: []*dst.Field{
						{
							Names: []*dst.Ident{dst.NewIdent("param")},
							Type:  dst.NewIdent("interface{}"),
						},
					},
				},
				Results: &dst.FieldList{},
			},
			Body: &dst.BlockStmt{
				List: tools.GoStringToStats(fmt.Sprintf("receiver.%s = param", EnhanceInstanceField)),
			},
		},
	}
}

func (i *InstanceEnhance) ReplaceFileContent(path, content string) string {
	return content
}
