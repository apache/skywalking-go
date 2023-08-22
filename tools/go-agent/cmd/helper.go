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

package main

import (
	"fmt"
	"os"
)

var version string

type EnhancementToolFlags struct {
	Help        bool   `swflag:"-h"`
	Debug       string `swflag:"-debug"`
	Config      string `swflag:"-config"`
	Inject      string `swflag:"-inject"`
	AllProjects bool   `swflag:"-all"`
	Version     bool   `swflag:"-version"`
}

func PrintUsageWithExit() {
	fmt.Printf(`Usage: go {build,install} -a [-work] -toolexec "%s" PACKAGE...

The Go-agent-enhance tool is designed for automatic enhancement of Golang programs, or inject the agent code into the project.

Options:
		-h
				Print the usage message.
		-inject
				Inject the agent code into the project, the value is the path of the project or single file.
		-all
				Inject the agent code into all the project in the current directory.
		-debug
				Helping to debug the enhance process, the value is the path of the debug file.
		-config
				The file path of the agent config file.
		-version
				Print current agent version.
`, os.Args[0])
	os.Exit(1)
}

func PrintVersion() {
	res := version
	if res == "" {
		res = "unknown"
	} else {
		res = fmt.Sprintf("v%s", res)
	}
	fmt.Printf("skywalking-go agent version: %s\n", res)
}
