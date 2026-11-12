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

func NewInvocation(receiver interface{}, args ...interface{}) Invocation {
	var argsCopy []interface{}
	if args != nil {
		argsCopy = make([]interface{}, len(args))
		copy(argsCopy, args)
	}
	return &realInvocation{
		callerInstance: receiver,
		args:           argsCopy,
		changeArgCallback: func(i int, i2 interface{}) {
			argsCopy[i] = i2
		},
	}
}

// InvokeIntercepted runs the interceptor around call in the same order as the
// code that the agent generates: method_intercept_before.tmpl,
// method_inserts.tmpl and method_intercept_after.tmpl. A panic in an
// interceptor is recovered, as in the generated code. It returns false when
// the before-hook skipped the call.
func InvokeIntercepted(interceptor Interceptor, invocation Invocation, call func() []interface{}) (called bool) {
	inv, ok := invocation.(*realInvocation)
	if !ok {
		panic("InvokeIntercepted needs an invocation from NewInvocation")
	}
	skip := func() (skip bool) {
		var mark interface{}
		depth := -1
		defer func() {
			if r := recover(); r != nil {
				rollbackActiveSpan(inv, mark, depth)
			}
		}()
		mark, depth = markActiveSpan()
		if err := interceptor.BeforeInvoke(inv); err != nil {
			rollbackActiveSpan(inv, mark, depth)
			return false
		}
		if inv.isContinue {
			inv.markExited()
			return true
		}
		return false
	}()
	if skip {
		return false
	}
	var results []interface{}
	defer func() {
		defer inv.markExited()
		defer func() {
			_ = recover()
		}()
		_ = interceptor.AfterInvoke(inv, results...)
	}()
	results = call()
	return true
}
