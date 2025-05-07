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
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"plugin-runner-helper/templates"
)

func main() {
	log.Printf("helper execute args: %v", os.Args)
	context, err := BuildContext()
	if err != nil {
		log.Fatalf("building context failure: %v", err)
	}

	// generate Dockerfile to build the plugin
	err = RenderDockerFile(context)
	if err != nil {
		log.Fatalf("build dockerfile failure: %v", err)
	}

	// generate scenarios.sh to start the plugin test case
	err = RenderScenariosScript(context)
	if err != nil {
		log.Fatalf("build scenarios failure: %v", err)
	}

	// generate docker-compose.yml to run the plugin
	err = RenderDockerCompose(context)
	if err != nil {
		log.Fatalf("build docker-compose failure: %v", err)
	}

	// generate validator.sh to validate the plugin
	err = RenderValidatorScript(context)
	if err != nil {
		log.Fatalf("build validator failure: %v", err)
	}

	// generate validator.sh to validate the plugin
	if context.IsWindows {
		err = RenderWSLScenariosScript(context)
		if err != nil {
			log.Fatalf("windows build wsl-scenarios failure: %v", err)
		}
	}
}

func RenderDockerFile(context *Context) error {
	_, v, found := strings.Cut(context.GoVersion, ".")
	if !found {
		return errors.New("invalid go version")
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return errors.New("invalid go version")
	}
	var greaterThanGo18 bool
	if i >= 18 {
		greaterThanGo18 = true
	}

	render, err := templates.Render("dockerfile.tpl", struct {
		ToolExecPath    string
		GreaterThanGo18 bool
		Context         *Context
	}{
		ToolExecPath:    strings.TrimPrefix(context.GoAgentPath, context.ProjectDir),
		GreaterThanGo18: greaterThanGo18,
		Context:         context,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(context.WorkSpaceDir, "Dockerfile"), []byte(render), 0o600)
}

func RenderScenariosScript(context *Context) error {
	tplName := "scenarios.tpl"
	dockerComposeFilePath := filepath.Join(context.WorkSpaceDir, "docker-compose.yml")
	if context.IsWindows {
		tplName = "windows-scenarios.tpl"
		dockerComposeFilePath = strings.ReplaceAll(dockerComposeFilePath, `\`, `/`)
	}

	render, err := templates.Render(tplName, struct {
		DockerComposeFilePath string
		Context               *Context
	}{
		DockerComposeFilePath: dockerComposeFilePath,
		Context:               context,
	})
	if err != nil {
		return err
	}
	render = strings.ReplaceAll(render, "\r\n", "\n")
	return os.WriteFile(filepath.Join(context.WorkSpaceDir, "scenarios.sh"), []byte(render), 0o600)
}

func RenderDockerCompose(context *Context) error {
	rel, err := filepath.Rel(context.ProjectDir, filepath.Join(context.WorkSpaceDir, "Dockerfile"))
	if err != nil {
		return err
	}

	dir := context.WorkSpaceDir

	tplName := "docker-compose.tpl"
	if context.IsWindows {
		tplName = "windows-docker-compose.tpl"
		context.WorkSpaceDir = "/root/repo/skywalking-go/test/plugins/workspace/" + context.ScenarioName + "/" + context.CaseName
	}

	render, err := templates.Render(tplName, struct {
		DockerFilePathRelateToProject string
		Context                       *Context
	}{
		DockerFilePathRelateToProject: rel,
		Context:                       context,
	})
	context.WorkSpaceDir = dir
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(context.WorkSpaceDir, "docker-compose.yml"), []byte(render), 0o600)
}

func RenderValidatorScript(context *Context) error {
	tplName := "validator.tpl"
	if context.IsWindows {
		tplName = "windows-validator.tpl"
	}
	render, err := templates.Render(tplName, struct {
		Context *Context
	}{
		Context: context,
	})
	if err != nil {
		return err
	}
	render = strings.ReplaceAll(render, "\r\n", "\n")
	return os.WriteFile(filepath.Join(context.WorkSpaceDir, "validator.sh"), []byte(render), 0o600)
}

func RenderWSLScenariosScript(context *Context) error {
	render, err := templates.Render("wsl-scenarios.tpl", struct {
		Context *Context
	}{
		Context: context,
	})
	if err != nil {
		return err
	}
	render = strings.ReplaceAll(render, "\r\n", "\n")
	return os.WriteFile(filepath.Join(context.WorkSpaceDir, "wsl-scenarios.sh"), []byte(render), 0o600)
}
