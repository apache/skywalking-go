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

import "github.com/apache/skywalking-go/plugins/core/operator"

func ParseFloat(val string, bitSize int) (float64, error) {
	if val == "" || bitSize == 0 {
		return 0, nil
	}
	op := operator.GetOperator()
	if op == nil {
		return 0, nil
	}
	return op.Tools().(operator.ToolsOperator).ParseFloat(val, bitSize)
}

func ParseBool(val string) bool {
	if val == "" {
		return false
	}
	op := operator.GetOperator()
	if op == nil {
		return false
	}
	return op.Tools().(operator.ToolsOperator).ParseBool(val)
}

func ParseInt(val string, base, bitSize int) (int64, error) {
	if val == "" || base == 0 || bitSize == 0 {
		return 0, nil
	}
	op := operator.GetOperator()
	if op == nil {
		return 0, nil
	}
	return op.Tools().(operator.ToolsOperator).ParseInt(val, base, bitSize)
}

func ParseStringArray(val string) ([]string, error) {
	if val == "" {
		return []string{}, nil
	}
	op := operator.GetOperator()
	if op == nil {
		return []string{}, nil
	}
	return op.Tools().(operator.ToolsOperator).ParseStringArray(val)
}

func Atoi(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	op := operator.GetOperator()
	if op == nil {
		return 0, nil
	}
	return op.Tools().(operator.ToolsOperator).Atoi(s)
}
