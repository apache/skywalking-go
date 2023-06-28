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

package consts

var (
	// DirectivePublic is the directive for generate the data as public of this package
	DirectivePublic = "//skywalking:public"

	// DirectiveReferenceGenerate is the directive for reference a generated type(corporation with public directive)
	DirectiveReferenceGenerate = "//skywalking:ref_generate"

	// DirectiveNative is the directive for reference to the native framework type
	// Usually used for reference the private types in the framework
	DirectiveNative = "//skywalking:native"

	// DirecitveNoCopy is the directive for define current go file would not copy to the framework
	DirecitveNoCopy = "//skywalking:nocopy"

	// DirectiveConfig is the directive for define the config variable
	DirectiveConfig = "//skywalking:config"

	// DirectiveInit is the directive for define the init function after the agent initialized
	DirectiveInit = "//skywalking:init"
)
