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

import "github.com/dave/dst"

// WithArgsCount filter methods with specific count of arguments.
func WithArgsCount(argsCount int) MethodFilterOption {
	return func(method *dst.FuncDecl, files []*dst.File) bool {
		return fieldListParameterCount(method.Type.Params) == argsCount
	}
}

// WithResultCount filter methods with specific count of results.
func WithResultCount(resultCount int) MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		return fieldListParameterCount(decl.Type.Results) == resultCount
	}
}

// WithArgType filter methods with specific type of the index of the argument.
func WithArgType(argIndex int, dataType string) MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		return verifyTypeSameInFieldList(decl.Type.Params, argIndex, dataType)
	}
}

// WithResultType filter methods with specific type of the index of the result.
func WithResultType(argIndex int, dataType string) MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		return verifyTypeSameInFieldList(decl.Type.Results, argIndex, dataType)
	}
}

// WithStaticMethod filter static methods.
func WithStaticMethod() MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		return decl.Recv == nil || len(decl.Recv.List) == 0
	}
}

// WithReceiverType filter methods with specific receiver type.
func WithReceiverType(dataType string) MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		if decl.Recv == nil || len(decl.Recv.List) == 0 {
			return false
		}
		return verifyTypeName(decl.Recv.List[0].Type, dataType)
	}
}

func fieldListParameterCount(fieldList *dst.FieldList) int {
	if fieldList == nil || len(fieldList.List) == 0 {
		return 0
	}
	res := 0
	for _, f := range fieldList.List {
		if len(f.Names) == 0 {
			res++
			continue
		}
		res += len(f.Names)
	}
	return res
}

func verifyTypeSameInFieldList(fieldList *dst.FieldList, inx int, typeStr string) bool {
	if inx >= fieldListParameterCount(fieldList) {
		return false
	}
	realInx := 0
	for _, f := range fieldList.List {
		if len(f.Names) == 0 {
			if realInx == inx {
				return verifyTypeName(f.Type, typeStr)
			}
			realInx++
			continue
		}
		for i := 0; i < len(f.Names); i++ {
			if realInx == inx {
				return verifyTypeName(f.Type, typeStr)
			}
			realInx++
		}
	}
	return false
}
