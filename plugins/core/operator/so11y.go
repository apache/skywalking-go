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

package operator

type So11yOperator interface {
	CollectErrorOfPlugin(pluginName string)
	GenNanoTime() int64
	CollectDurationOfInterceptor(costTime int64)
}

func ErrorOfPlugin(pluginName string) {
	op := GetOperator()
	if op == nil {
		return
	}
	op.(So11yOperator).CollectErrorOfPlugin(pluginName)
}

func NanoTime() int64 {
	op := GetOperator()
	if op == nil {
		return 0
	}
	return op.(So11yOperator).GenNanoTime()
}

func DurationOfInterceptor(costTime int64) {
	op := GetOperator()
	if op == nil {
		return
	}
	op.(So11yOperator).CollectDurationOfInterceptor(costTime)
}
