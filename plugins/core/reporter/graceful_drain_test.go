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
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"

	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
)

// TestCollectStreamSurvivesGracefulStopWithRefusedReconnect: GracefulStop while Collect is open, then a reconnect attempt
// is refused (TransientFailure). The draining stream is still usable.
func TestCollectStreamSurvivesGracefulStopWithRefusedReconnect(t *testing.T) {
	srv := &countingTraceServer{}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer()
	agentv3.RegisterTraceSegmentReportServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()

	addr := lis.Addr().String()
	cm, err := NewConnectionManager(nil, time.Second, addr, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cm.Close()
	conn, err := cm.GetConnection(addr)
	if err != nil {
		t.Fatal(err)
	}
	client := agentv3.NewTraceSegmentReportServiceClient(conn)
	ctx, cancel, stopOpen := BackendStreamContext(time.Second)
	defer cancel()
	stream, err := client.Collect(metadata.NewOutgoingContext(ctx, cm.GetMD()))
	if timedOut := stopOpen(); err != nil || timedOut {
		t.Fatalf("open: %v %v", err, timedOut)
	}

	if err := stream.Send(&agentv3.SegmentObject{TraceId: "d1"}); err != nil {
		t.Fatalf("first Send: %v", err)
	}
	go gs.GracefulStop() // GOAWAY + listener closed; waits for our stream
	waitFor(t, func() bool { return conn.GetState() == connectivity.Idle }, 5*time.Second)
	conn.Connect() // stands in for the next heartbeat RPC
	waitFor(t, func() bool { return conn.GetState() == connectivity.TransientFailure }, 5*time.Second)
	time.Sleep(200 * time.Millisecond)

	if err := stream.Send(&agentv3.SegmentObject{TraceId: "d2"}); err != nil {
		t.Fatalf("second Send on draining stream: %v", err)
	}
	if _, err := stream.CloseAndRecv(); err != nil {
		t.Fatalf("CloseAndRecv on draining stream: %v", err)
	}
	if got := srv.count.Load(); got != 2 {
		t.Fatalf("server received %d segments, want 2", got)
	}
}
