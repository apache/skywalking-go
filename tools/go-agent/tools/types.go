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

var basicDataTypes = make(map[string]bool)

func init() {
	types := []string{
		"bool", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "int", "uint", "uintptr",
		"float32", "float64", "complex64", "complex128", "string", "error", "interface{}", "_", "byte", "any",
	}
	for _, tp := range types {
		basicDataTypes[tp] = true
	}
}

// nolint
func IsBasicDataType(name string) bool {
	return basicDataTypes[name]
}
