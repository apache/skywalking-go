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

type realInvocation struct {
	callerInstance interface{}
	args           []interface{}

	changeArgCallback func(int, interface{})

	isContinue   bool
	returnValues []interface{}

	context interface{}

	// self obs
	interTimeCost    int64
	beforeInterStart int64
}

func (i *realInvocation) CallerInstance() interface{} {
	return i.callerInstance
}

func (i *realInvocation) Args() []interface{} {
	return i.args
}

func (i *realInvocation) ChangeArg(index int, newValue interface{}) {
	i.changeArgCallback(index, newValue)
}

func (i *realInvocation) IsContinue() bool {
	return i.isContinue
}

func (i *realInvocation) DefineReturnValues(values ...interface{}) {
	i.isContinue = true
	i.returnValues = values
}

func (i *realInvocation) SetContext(context interface{}) {
	i.context = context
}

func (i *realInvocation) GetContext() interface{} {
	return i.context
}
