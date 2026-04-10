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

package instrument

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTypeNameByExp_BasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		expr     dst.Expr
		expected string
	}{
		{"ident", &dst.Ident{Name: "string"}, "string"},
		{"selector", &dst.SelectorExpr{X: dst.NewIdent("context"), Sel: dst.NewIdent("Context")}, "context.Context"},
		{"star selector", &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("http"), Sel: dst.NewIdent("Request")}}, "*http.Request"},
		{"star ident", &dst.StarExpr{X: dst.NewIdent("error")}, "*error"},
		{"ellipsis", &dst.Ellipsis{Elt: dst.NewIdent("string")}, "...string"},
		{"array", &dst.ArrayType{Elt: dst.NewIdent("int")}, "[]int"},
		{"interface", &dst.InterfaceType{}, "interface{}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, generateTypeNameByExp(tt.expr))
		})
	}
}

func TestGenerateTypeNameByExp_FuncType(t *testing.T) {
	tests := []struct {
		name     string
		expr     dst.Expr
		expected string
	}{
		{
			"func with no params no results",
			&dst.FuncType{
				Params: &dst.FieldList{},
			},
			"func()",
		},
		{
			"func with single param",
			&dst.FuncType{
				Params: &dst.FieldList{List: []*dst.Field{
					{Type: dst.NewIdent("int")},
				}},
			},
			"func(int)",
		},
		{
			"func with multiple params",
			&dst.FuncType{
				Params: &dst.FieldList{List: []*dst.Field{
					{Type: &dst.SelectorExpr{X: dst.NewIdent("context"), Sel: dst.NewIdent("Context")}},
					{Type: dst.NewIdent("string")},
				}},
			},
			"func(context.Context, string)",
		},
		{
			"func with single unnamed result",
			&dst.FuncType{
				Params: &dst.FieldList{List: []*dst.Field{
					{Type: dst.NewIdent("int")},
				}},
				Results: &dst.FieldList{List: []*dst.Field{
					{Type: dst.NewIdent("error")},
				}},
			},
			"func(int) error",
		},
		{
			"func with multiple results",
			&dst.FuncType{
				Params: &dst.FieldList{List: []*dst.Field{
					{Type: dst.NewIdent("string")},
				}},
				Results: &dst.FieldList{List: []*dst.Field{
					{Type: dst.NewIdent("int")},
					{Type: dst.NewIdent("error")},
				}},
			},
			"func(string) (int, error)",
		},
		{
			"func with complex params",
			&dst.FuncType{
				Params: &dst.FieldList{List: []*dst.Field{
					{Type: &dst.SelectorExpr{X: dst.NewIdent("context"), Sel: dst.NewIdent("Context")}},
					{Type: &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("primitive"), Sel: dst.NewIdent("SendResult")}}},
					{Type: dst.NewIdent("error")},
				}},
			},
			"func(context.Context, *primitive.SendResult, error)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, generateTypeNameByExp(tt.expr))
		})
	}
}

func TestGenerateTypeNameByExp_ChanType(t *testing.T) {
	tests := []struct {
		name     string
		expr     dst.Expr
		expected string
	}{
		{
			"bidirectional chan",
			&dst.ChanType{
				Dir:   dst.SEND | dst.RECV,
				Value: dst.NewIdent("int"),
			},
			"chan int",
		},
		{
			"send-only chan",
			&dst.ChanType{
				Dir:   dst.SEND,
				Value: dst.NewIdent("int"),
			},
			"chan<- int",
		},
		{
			"receive-only chan",
			&dst.ChanType{
				Dir:   dst.RECV,
				Value: dst.NewIdent("Delivery"),
			},
			"<-chan Delivery",
		},
		{
			"receive-only chan with selector type",
			&dst.ChanType{
				Dir:   dst.RECV,
				Value: &dst.SelectorExpr{X: dst.NewIdent("amqp"), Sel: dst.NewIdent("Delivery")},
			},
			"<-chan amqp.Delivery",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, generateTypeNameByExp(tt.expr))
		})
	}
}
