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

type ReflectFieldFilter interface {
	Apply(interface{})
}

// GetInstanceValueByType using reflect to get the first value of instance by type
func GetInstanceValueByType(instance interface{}, filters ...ReflectFieldFilter) interface{} {
	if instance == nil || len(filters) == 0 {
		return nil
	}
	op := operator.GetOperator()
	if op == nil {
		return nil
	}
	fs := make([]interface{}, len(filters))
	for i, f := range filters {
		fs[i] = f
	}
	return op.Tools().(operator.ToolsOperator).ReflectGetValue(instance, fs)
}

func WithFieldName(name string) ReflectFieldFilter {
	return buildFieldFilterOption(func(s ReflectFieldFilterSetter) {
		s.SetName(name)
	})
}

func WithInterfaceType(typeVal interface{}) ReflectFieldFilter {
	return buildFieldFilterOption(func(s ReflectFieldFilterSetter) {
		s.SetInterfaceType(typeVal)
	})
}

func WithType(typeVal interface{}) ReflectFieldFilter {
	return buildFieldFilterOption(func(s ReflectFieldFilterSetter) {
		s.SetType(typeVal)
	})
}
