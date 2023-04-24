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
		return (method.Type.Params == nil && len(method.Type.Params.List) == argsCount) || (len(method.Type.Params.List) == argsCount)
	}
}

// WithResultCount filter methods with specific count of results.
func WithResultCount(resultCount int) MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		return (decl.Type.Results == nil && resultCount == 0) || (len(decl.Type.Results.List) == resultCount)
	}
}

// WithArgType filter methods with specific type of the index of the argument.
func WithArgType(argIndex int, dataType string) MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		if decl.Type.Params == nil || len(decl.Type.Params.List) <= argIndex {
			return false
		}
		return verifyTypeName(decl.Type.Params.List[argIndex].Type, dataType)
	}
}

// WithResultType filter methods with specific type of the index of the result.
func WithResultType(argIndex int, dataType string) MethodFilterOption {
	return func(decl *dst.FuncDecl, files []*dst.File) bool {
		if decl.Type.Results == nil || len(decl.Type.Results.List) <= argIndex {
			return false
		}
		return verifyTypeName(decl.Type.Results.List[argIndex].Type, dataType)
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
