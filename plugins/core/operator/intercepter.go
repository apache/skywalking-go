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

type Invocation struct {
	// CallerInstance is the instance of the caller, nil if the method is static method.
	CallerInstance interface{}
	// Args is the arguments data of the method, please cast to the specific type to get more inforamtion.
	Args []interface{}

	// Continue is the flag to control the method invocation, if it is false, the target method would not be invoked.
	Continue bool
	// Return is the return data of the method, the result must be provided when continue the target method invoke.
	// and the result size must be the same as the method return value size.
	Return []interface{}

	// Context is the customized context of the method invocation, it should be propagated the tracing span.
	Context interface{}
}

type EnhancedInstance interface {
	// GetSkyWalkingDynamicField get the customized data from instance
	GetSkyWalkingDynamicField() interface{}
	// SetSkyWalkingDynamicField set the customized data into the instance
	SetSkyWalkingDynamicField(interface{})
}

type Interceptor interface {
	// BeforeInvoke would be called before the target method invocation.
	BeforeInvoke(invocation *Invocation) error
	// AfterInvoke would be called after the target method invocation.
	AfterInvoke(invocation *Invocation, result ...interface{}) error
}
