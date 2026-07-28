// Licensed to the Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with this work for
// additional information regarding copyright ownership. The ASF licenses this
// file to you under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderDockerFileRefreshesWorkspaceGoDirective(t *testing.T) {
	tests := []struct {
		goVersion string
		want      bool
	}{
		{goVersion: "1.17", want: false},
		{goVersion: "1.18", want: true},
		{goVersion: "1.25", want: true},
		{goVersion: "1.26", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.goVersion, func(t *testing.T) {
			projectDir := t.TempDir()
			workspaceDir := filepath.Join(projectDir, "workspace")
			if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
				t.Fatal(err)
			}
			context := &Context{
				WorkSpaceDir: workspaceDir,
				ProjectDir:   projectDir,
				GoVersion:    tt.goVersion,
				ScenarioName: "grpc",
				CaseName:     "compatibility",
				GoAgentPath:  filepath.Join(projectDir, "agent"),
				Config:       &Config{StartScript: "./bin/startup.sh"},
			}

			if err := RenderDockerFile(context); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(workspaceDir, "Dockerfile"))
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Contains(string(content), "RUN go mod tidy\n# go mod tidy") &&
				strings.Contains(string(content), "RUN go work use .")
			if got != tt.want {
				t.Fatalf("workspace refresh present = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderScenariosAcceptsModernComposeMajorVersions(t *testing.T) {
	workspaceDir := t.TempDir()
	context := &Context{
		WorkSpaceDir: workspaceDir,
		ScenarioName: "http",
	}

	if err := RenderScenariosScript(context); err != nil {
		t.Fatal(err)
	}
	if err := RenderWSLScenariosScript(context); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"scenarios.sh", "wsl-scenarios.sh"} {
		content, err := os.ReadFile(filepath.Join(workspaceDir, name))
		if err != nil {
			t.Fatal(err)
		}
		script := string(content)
		for _, want := range []string{
			`compose_major=${compose_version#v}`,
			`compose_major=${compose_major%%.*}`,
			`(( compose_major >= 2 ))`,
		} {
			if !strings.Contains(script, want) {
				t.Fatalf("generated %s does not contain %q", name, want)
			}
		}
	}
}
