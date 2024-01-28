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

package tools

import (
	"testing"

	"github.com/dave/dst"
)

func buildParameterValidateInfo(name, typeName string) *ParameterInfo {
	return &ParameterInfo{
		Name:     name,
		TypeName: typeName,
	}
}

type TestEnhanceParameterInfo struct {
	funcCode string
	recvs    []*ParameterInfo
	params   []*ParameterInfo
	results  []*ParameterInfo
}

func TestEnhanceParameterNames(t *testing.T) {
	tests := []TestEnhanceParameterInfo{
		{
			funcCode: `func (*Example) Test(int) bool {
				return false
			}`,
			recvs: []*ParameterInfo{
				buildParameterValidateInfo("skywalking_recv_0", "*Example"),
			},
			params: []*ParameterInfo{
				buildParameterValidateInfo("skywalking_param_0", "int"),
			},
			results: []*ParameterInfo{
				buildParameterValidateInfo("skywalking_result_0", "bool"),
			},
		},
		{
			funcCode: `func (e *Example) Test(i int) (b bool) {
				return false
}`,
			recvs: []*ParameterInfo{
				buildParameterValidateInfo("e", "*Example"),
			},
			params: []*ParameterInfo{
				buildParameterValidateInfo("i", "int"),
			},
			results: []*ParameterInfo{
				buildParameterValidateInfo("b", "bool"),
			},
		},
	}

	validateParameterTestList(t, tests)
}

func TestEnhanceParameterNamesMultiParams(t *testing.T) {
	tests := []TestEnhanceParameterInfo{
		{
			funcCode: `func (*Example) Test(n, m int) bool {
				return false
			}`,
			recvs: []*ParameterInfo{
				buildParameterValidateInfo("skywalking_recv_0", "*Example"),
			},
			params: []*ParameterInfo{
				buildParameterValidateInfo("n", "int"),
				buildParameterValidateInfo("m", "int"),
			},
			results: []*ParameterInfo{
				buildParameterValidateInfo("skywalking_result_0", "bool"),
			},
		},
		{
			funcCode: `func (e *Example) Test(n, m int) (b bool) {
				return false
}`,
			recvs: []*ParameterInfo{
				buildParameterValidateInfo("e", "*Example"),
			},
			params: []*ParameterInfo{
				buildParameterValidateInfo("n", "int"),
				buildParameterValidateInfo("m", "int"),
			},
			results: []*ParameterInfo{
				buildParameterValidateInfo("b", "bool"),
			},
		},
	}

	validateParameterTestList(t, tests)
}

func validateParameterTestList(t *testing.T, tests []TestEnhanceParameterInfo) {
	for i, test := range tests {
		fun := GoStringToDecls(test.funcCode)[0].(*dst.FuncDecl)
		var actualRecv, actualParams, actualResults []*ParameterInfo
		if fun.Recv != nil {
			actualRecv = EnhanceParameterNames(fun.Recv, FieldListTypeRecv)
		}
		actualParams = EnhanceParameterNames(fun.Type.Params, FieldListTypeParam)
		actualResults = EnhanceParameterNames(fun.Type.Results, FieldListTypeResult)

		validateParameterInfo(t, i, FieldListTypeRecv, actualRecv, test.recvs)
		validateParameterInfo(t, i, FieldListTypeParam, actualParams, test.params)
		validateParameterInfo(t, i, FieldListTypeResult, actualResults, test.results)
	}
}

func validateParameterInfo(t *testing.T, inx int, flistType FieldListType, actual, excepted []*ParameterInfo) {
	if len(actual) != len(excepted) {
		t.Errorf("case %d:%s: expected count %d , actual %d", inx, flistType, len(excepted), len(actual))
	}
	for i, exp := range excepted {
		act := actual[i]
		if exp.Name != act.Name {
			t.Errorf("case %d:%s: expected name %s , actual %s", inx, flistType, exp.Name, act.Name)
		}
		if exp.TypeName != act.TypeName {
			t.Errorf("case %d:%s: expected type %s , actual %s", inx, flistType, exp.TypeName, act.TypeName)
		}
	}
}
