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

package reporter

import (
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

// TestGeneratedInitManagerPropagatesConnectionError ensures the TLS branch does
// not shadow the outer err used by NewConnectionManager. Previously `tc, err :=`
// left outer err nil after a backend parse failure, and a later nil-pointer
// GetConnection panic replaced the intended error return.
//
// Lint cannot see variable scopes inside a template string, so this renders
// initManagerFunc, stubs the constructors, and runs the emitted Go for TLS off
// and on via SW_AGENT_REPORTER_GRPC_TLS_ENABLE.
func TestGeneratedInitManagerPropagatesConnectionError(t *testing.T) {
	if err := config.LoadConfig(""); err != nil {
		t.Fatal(err)
	}
	generated := html.UnescapeString(tools.ExecuteTemplate(initManagerFunc, struct {
		Config *config.Config
	}{Config: config.GetConfig()}))

	if strings.Contains(generated, "tc, err :=") {
		t.Fatal("TLS credential error must not shadow NewConnectionManager err")
	}
	if !strings.Contains(generated, "tc, tlsErr :=") {
		t.Fatal("expected tlsErr binding for generateTLSCredential")
	}
	if !strings.Contains(generated, "SW_AGENT_REPORTER_GRPC_TLS_ENABLE") {
		t.Fatal("expected TLS enable to be driven by env at runtime")
	}

	// Exercise the emitted Go code: lint cannot inspect variable scopes inside
	// a template string. Stub external constructors to isolate error propagation.
	generated = strings.ReplaceAll(generated, "operator.LogOperator", "interface{}")
	source := "package main\nimport (\"fmt\"; \"os\"; \"strconv\"; \"strings\"; \"time\")\n" +
		generated + initManagerErrorHarness
	sourcePath := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated initManager failed to propagate connection error: %v\n%s", err, output)
	}
}

const initManagerErrorHarness = `
type ConnectionManager struct{}
type CDSManager struct{}
type PprofTaskManager struct{}

var connectionError = fmt.Errorf("no valid backend service addresses")

func generateTLSCredential(string, string, string, bool) (interface{}, error) {
	return struct{}{}, nil
}

func NewConnectionManager(interface{}, time.Duration, string, string, interface{}) (*ConnectionManager, error) {
	return nil, connectionError
}

func NewCDSManager(interface{}, string, time.Duration, *ConnectionManager) (*CDSManager, error) {
	panic("CDS must not start after connection initialization fails")
}

func NewPprofTaskManager(interface{}, string, time.Duration, *ConnectionManager, string) (*PprofTaskManager, error) {
	panic("pprof must not start after connection initialization fails")
}

func main() {
	for _, tlsEnabled := range []string{"false", "true"} {
		if err := os.Setenv("SW_AGENT_REPORTER_GRPC_TLS_ENABLE", tlsEnabled); err != nil {
			panic(err)
		}
		conn, cds, pprof, err := initManager(nil, time.Second)
		if err != connectionError || conn != nil || cds != nil || pprof != nil {
			panic(fmt.Sprintf("TLS=%s: connection error was not propagated: %v", tlsEnabled, err))
		}
	}
}
`
