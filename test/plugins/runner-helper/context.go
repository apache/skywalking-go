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
	"flag"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var workSpaceDir = flag.String("workspace", "", "testcase workspace directory")
var projectDir = flag.String("project", "", "project directory")
var goVersion = flag.String("go-version", "", "go version")
var scenarioName = flag.String("scenario", "", "scenario name")
var caseName = flag.String("case", "", "case name")
var goAgentPath = flag.String("go-agent", "", "go agent file path")
var debugMode = flag.String("debug", "", "is debug mode")

func BuildContext() (*Context, error) {
	flag.Parse()
	var err error
	err = flagValueCannotBeEmpty(err, workSpaceDir, "workspace directory cannot be empty")
	err = flagValueCannotBeEmpty(err, projectDir, "project directory cannot be empty")
	err = flagValueCannotBeEmpty(err, goVersion, "go version cannot be empty")
	err = flagValueCannotBeEmpty(err, scenarioName, "scenario name cannot be empty")
	err = flagValueCannotBeEmpty(err, caseName, "case name cannot be empty")
	err = flagValueCannotBeEmpty(err, goAgentPath, "go agent path cannot be empty")
	if err != nil {
		return nil, err
	}

	config, err := loadConfig(filepath.Join(*workSpaceDir, "plugin.yml"))
	if err != nil {
		return nil, err
	}

	return &Context{
		WorkSpaceDir: filepath.Clean(*workSpaceDir),
		ProjectDir:   filepath.Clean(*projectDir),
		GoVersion:    *goVersion,
		ScenarioName: *scenarioName,
		CaseName:     *caseName,
		GoAgentPath:  filepath.Clean(*goAgentPath),
		DebugMode:    *debugMode == "on",
		Config:       config,
	}, nil
}

type Context struct {
	WorkSpaceDir string
	ProjectDir   string
	GoVersion    string
	ScenarioName string
	CaseName     string
	GoAgentPath  string
	DebugMode    bool
	Config       *Config
}

type Config struct {
	EntryService   string           `yaml:"entry-service"`
	HealthChecker  string           `yaml:"health-checker"`
	StartScript    string           `yaml:"start-script"`
	FrameworkName  string           `yaml:"framework"`
	ExportPort     int              `yaml:"export-port"`
	SupportVersion []SupportVersion `yaml:"support-version"`
}

type SupportVersion struct {
	GoVersion  string   `yaml:"go"`
	Frameworks []string `yaml:"framework"`
}

func loadConfig(path string) (config *Config, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	err = yaml.Unmarshal(content, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func flagValueCannotBeEmpty(err error, val *string, msg string) error {
	if err != nil {
		return err
	}
	if *val == "" {
		return errors.New(msg)
	}
	return nil
}
