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
	"strings"
	"testing"

	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

// TestGeneratedInitManagerPropagatesConnectionError ensures the TLS branch does
// not shadow the outer err used by NewConnectionManager. Previously `tc, err :=`
// left outer err nil after a backend parse failure, and a later nil-pointer
// GetConnection panic replaced the intended error return.
func TestGeneratedInitManagerPropagatesConnectionError(t *testing.T) {
	cfg := &config.Config{}
	cfg.Reporter.GRPC.Authentication.UnmarshalString("")
	cfg.Reporter.GRPC.BackendService.UnmarshalString("")
	cfg.Reporter.GRPC.TLS.Enable.UnmarshalString("true")
	cfg.Reporter.GRPC.TLS.CAPath.UnmarshalString("/nonexistent/ca.crt")
	cfg.Reporter.GRPC.TLS.ClientKeyPath.UnmarshalString("")
	cfg.Reporter.GRPC.TLS.ClientCertChainPath.UnmarshalString("")
	cfg.Reporter.GRPC.TLS.InsecureSkipVerify.UnmarshalString("false")
	cfg.Reporter.GRPC.CDSFetchInterval.UnmarshalString("20")
	cfg.Reporter.GRPC.Pprof.PprofFetchInterval.UnmarshalString("20")
	cfg.Reporter.GRPC.Pprof.PprofFilePath.UnmarshalString("/tmp")

	generated := html.UnescapeString(tools.ExecuteTemplate(initManagerFunc, struct {
		Config *config.Config
	}{Config: cfg}))

	if strings.Contains(generated, "tc, err :=") {
		t.Fatal("TLS credential error must not shadow NewConnectionManager err")
	}
	if !strings.Contains(generated, "tc, tlsErr :=") {
		t.Fatal("expected tlsErr binding for generateTLSCredential")
	}
	if !strings.Contains(generated, "if tlsErr != nil") {
		t.Fatal("expected tlsErr check before NewConnectionManager")
	}
	// Outer err must still gate the success path after dial/manager setup.
	if !strings.Contains(generated, "connManager, err = NewConnectionManager") {
		t.Fatal("expected NewConnectionManager assignment into outer err")
	}
	if !strings.Contains(generated, "if err != nil") {
		t.Fatal("expected outer err propagation after NewConnectionManager")
	}
}
