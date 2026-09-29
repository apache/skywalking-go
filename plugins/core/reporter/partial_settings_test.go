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
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
)

// TestMultiBackendPerAddrDialTimeoutFailsOverPastPartialSettings proves a first
// peer that sends a SETTINGS frame header (6-byte payload declared) and then
// withholds the payload cannot starve the standby.
func TestMultiBackendPerAddrDialTimeoutFailsOverPastPartialSettings(t *testing.T) {
	partial, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Close()
	go func() {
		for {
			c, acceptErr := partial.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				go func() { _, _ = io.Copy(io.Discard, c) }()
				// 9-byte header: length=6, type=SETTINGS(0x4), flags=0, stream=0.
				_, _ = c.Write([]byte{0, 0, 6, 0x4, 0, 0, 0, 0, 0})
			}(c)
		}
	}()

	healthySrv := &countingTraceServer{}
	healthyLis, healthyGS := serveTrace(t, healthySrv)
	defer healthyLis.Close()
	defer healthyGS.Stop()

	partialAddr := partial.Addr().String()
	healthyAddr := healthyLis.Addr().String()

	var cm *ConnectionManager
	var conn *grpc.ClientConn
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		backends := partialAddr + "," + healthyAddr
		cm, err = NewConnectionManager(nil, time.Second, backends, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		conn, err = cm.GetConnection(backends)
		if err != nil {
			t.Fatal(err)
		}
		var resolved []string
		for i := 0; i < 150 && len(resolved) != 2; i++ {
			resolved = resolvedBackendAddressesForTest(cm)
			time.Sleep(20 * time.Millisecond)
		}
		if len(resolved) == 2 && resolved[0] == partialAddr {
			break
		}
		cm.Close()
		cm, conn = nil, nil
	}
	if cm == nil {
		t.Fatal("could not get partial peer first")
	}
	defer cm.Close()

	waitFor(t, func() bool { return conn.GetState() == connectivity.Ready }, 13*time.Second)
	client := agentv3.NewTraceSegmentReportServiceClient(conn)
	if probeErr := sendTraceProbe(client, cm.GetMD(), "partial-settings"); probeErr != nil {
		t.Fatalf("probe: %v", probeErr)
	}
}
