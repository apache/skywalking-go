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
	"log"
	"os"
	"os/exec"

	"github.com/apache/skywalking-go/tools/go-agent-enhance/instrument"
	"github.com/apache/skywalking-go/tools/go-agent-enhance/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent-enhance/tools"
)

var toolFlags = &EnhancementToolFlags{}

func main() {
	args := os.Args[1:]
	// Print usage
	if err := tools.ParseFlags(toolFlags, args); err != nil || toolFlags.Help {
		PrintUsageWithExit()
	}

	// only enhance the "compile" phase
	cmdName := tools.ParseProxyCommandName(args)
	if cmdName != "compile" {
		executeDelegateCommand(args)
		return
	}

	// parse the args
	compileOptions := &api.CompileOptions{}
	if err := tools.ParseFlags(compileOptions, args); err != nil {
		executeDelegateCommand(args)
		return
	}

	// execute the enhancement
	args, err := instrument.Execute(compileOptions, args)
	if err != nil {
		log.Fatal(err)
	}

	// execute the delegate command with updated args
	executeDelegateCommand(args)
}

func executeDelegateCommand(args []string) {
	path := args[0]
	args = args[1:]
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if e := cmd.Run(); e != nil {
		log.Fatal(e)
	}
}
