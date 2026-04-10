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
	"embed"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
)

func TestInstrument_tryToFindThePluginVersion(t *testing.T) {
	tests := []struct {
		name string
		opts *api.CompileOptions
		ins  instrument.Instrument
		want string
	}{
		{
			"normal plugin path",
			&api.CompileOptions{
				AllArgs: []string{
					"github.com/gin-gonic/gin@1.1.1/gin.go",
				},
			},
			NewTestInstrument("github.com/gin-gonic/gin"),
			"1.1.1",
		},
		{
			"plugin with upper-case path",
			&api.CompileOptions{
				AllArgs: []string{
					"github.com/!shopify/sarama@1.34.1/acl.go",
				},
			},
			NewTestInstrument("github.com/Shopify/sarama"),
			"1.34.1",
		},
		{
			"plugin for go stdlib",
			&api.CompileOptions{
				AllArgs: []string{
					"/opt/homebrew/Cellar/go/1.21.4/libexec/src/runtime/metrics/sample.go",
				},
			},
			NewTestInstrument("runtime/metrics"),
			"",
		},
		{
			"plugin for replaced module",
			&api.CompileOptions{
				AllArgs: []string{
					"/home/user/skywalking-go/toolkit/trace/api.go",
				},
			},
			NewTestInstrument("github.com/apache/skywalking-go/toolkit"),
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &Instrument{}
			got, err := i.tryToFindThePluginVersion(tt.opts, tt.ins)
			if err != nil {
				require.NoError(t, err)
			}
			if got != tt.want {
				t.Errorf("tryToFindThePluginVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}

type TestInstrument struct {
	basePackage string
}

func NewTestInstrument(basePackage string) *TestInstrument {
	return &TestInstrument{basePackage: basePackage}
}

func (i *TestInstrument) Name() string {
	return ""
}

func (i *TestInstrument) BasePackage() string {
	return i.basePackage
}

func (i *TestInstrument) VersionChecker(version string) bool {
	return true
}

func (i *TestInstrument) Points() []*instrument.Point {
	return []*instrument.Point{}
}

func (i *TestInstrument) FS() *embed.FS {
	return nil
}

func TestInstrument_validateMethodInsMatch_WithArgTypeFilters(t *testing.T) {
	inst := &Instrument{}

	handleStreamV1 := &dst.FuncDecl{
		Name: dst.NewIdent("handleStream"),
		Recv: &dst.FieldList{List: []*dst.Field{
			{Type: &dst.StarExpr{X: dst.NewIdent("Server")}},
		}},
		Type: &dst.FuncType{
			Params: &dst.FieldList{List: []*dst.Field{
				{Type: &dst.SelectorExpr{X: dst.NewIdent("transport"), Sel: dst.NewIdent("ServerTransport")}},
				{Type: &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("transport"), Sel: dst.NewIdent("Stream")}}},
			}},
		},
	}

	handleStreamV2 := &dst.FuncDecl{
		Name: dst.NewIdent("handleStream"),
		Recv: &dst.FieldList{List: []*dst.Field{
			{Type: &dst.StarExpr{X: dst.NewIdent("Server")}},
		}},
		Type: &dst.FuncType{
			Params: &dst.FieldList{List: []*dst.Field{
				{Type: &dst.SelectorExpr{X: dst.NewIdent("transport"), Sel: dst.NewIdent("ServerTransport")}},
				{Type: &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("transport"), Sel: dst.NewIdent("ServerStream")}}},
			}},
		},
	}

	matcherV1 := &instrument.EnhanceMatcher{
		Type:     instrument.EnhanceTypeMethod,
		Name:     "handleStream",
		Receiver: "*Server",
		MethodFilters: []instrument.MethodFilterOption{
			instrument.WithArgType(0, "transport.ServerTransport"),
			instrument.WithArgType(1, "*transport.Stream"),
		},
	}

	matcherV2 := &instrument.EnhanceMatcher{
		Type:     instrument.EnhanceTypeMethod,
		Name:     "handleStream",
		Receiver: "*Server",
		MethodFilters: []instrument.MethodFilterOption{
			instrument.WithArgType(0, "transport.ServerTransport"),
			instrument.WithArgType(1, "*transport.ServerStream"),
		},
	}

	tests := []struct {
		name    string
		matcher *instrument.EnhanceMatcher
		node    *dst.FuncDecl
		want    bool
	}{
		{"V1 matcher matches V1 signature", matcherV1, handleStreamV1, true},
		{"V1 matcher does not match V2 signature", matcherV1, handleStreamV2, false},
		{"V2 matcher does not match V1 signature", matcherV2, handleStreamV1, false},
		{"V2 matcher matches V2 signature", matcherV2, handleStreamV2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inst.validateMethodInsMatch(tt.matcher, tt.node, nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInstrument_validateMethodInsMatch_WithResultTypeFilter(t *testing.T) {
	inst := &Instrument{}

	sendResponseDecl := &dst.FuncDecl{
		Name: dst.NewIdent("sendResponse"),
		Recv: &dst.FieldList{List: []*dst.Field{
			{Type: &dst.StarExpr{X: dst.NewIdent("Server")}},
		}},
		Type: &dst.FuncType{
			Params: &dst.FieldList{List: []*dst.Field{}},
			Results: &dst.FieldList{List: []*dst.Field{
				{Type: dst.NewIdent("error")},
			}},
		},
	}

	matcher := &instrument.EnhanceMatcher{
		Type:     instrument.EnhanceTypeMethod,
		Name:     "sendResponse",
		Receiver: "*Server",
		MethodFilters: []instrument.MethodFilterOption{
			instrument.WithResultType(0, "error"),
		},
	}

	assert.True(t, inst.validateMethodInsMatch(matcher, sendResponseDecl, nil))
}

func TestInstrument_validateMethodInsMatch_ReceiverMismatch(t *testing.T) {
	inst := &Instrument{}

	methodDecl := &dst.FuncDecl{
		Name: dst.NewIdent("handleStream"),
		Recv: &dst.FieldList{List: []*dst.Field{
			{Type: &dst.StarExpr{X: dst.NewIdent("ClientConn")}},
		}},
		Type: &dst.FuncType{
			Params: &dst.FieldList{List: []*dst.Field{
				{Type: &dst.SelectorExpr{X: dst.NewIdent("transport"), Sel: dst.NewIdent("ServerTransport")}},
				{Type: &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("transport"), Sel: dst.NewIdent("Stream")}}},
			}},
		},
	}

	matcher := &instrument.EnhanceMatcher{
		Type:     instrument.EnhanceTypeMethod,
		Name:     "handleStream",
		Receiver: "*Server",
		MethodFilters: []instrument.MethodFilterOption{
			instrument.WithArgType(0, "transport.ServerTransport"),
			instrument.WithArgType(1, "*transport.Stream"),
		},
	}

	assert.False(t, inst.validateMethodInsMatch(matcher, methodDecl, nil))
}
