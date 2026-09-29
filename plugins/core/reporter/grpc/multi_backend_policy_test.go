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
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpc

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	commonv3 "github.com/apache/skywalking-go/protocols/collect/common/v3"
	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
	managementv3 "github.com/apache/skywalking-go/protocols/collect/management/v3"
)

type failingAcknowledgementServer struct {
	agentv3.UnimplementedTraceSegmentReportServiceServer
	code  codes.Code
	count atomic.Int32
	calls atomic.Int32
}

func (s *failingAcknowledgementServer) Collect(stream agentv3.TraceSegmentReportService_CollectServer) error {
	s.calls.Add(1)
	_, err := stream.Recv()
	if err != nil {
		return err
	}
	s.count.Add(1)
	// Fail while the client stream is still active (not only after CloseAndRecv).
	return status.Error(s.code, "collector failed after receiving the segment")
}

func serveFailingAcknowledgements(t *testing.T, server *failingAcknowledgementServer) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer()
	agentv3.RegisterTraceSegmentReportServiceServer(gs, server)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	t.Cleanup(func() { _ = lis.Close() })
	return lis.Addr().String()
}

// Unified long-lived Collect must not replay segments after an RPC error and must
// keep the shared ClientConn (no recreate).
func TestMultiBackendDoesNotReplayOrRotateOnRPCError(t *testing.T) {
	for _, code := range []codes.Code{
		codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted,
		codes.Unauthenticated, codes.PermissionDenied,
	} {
		t.Run(code.String(), func(t *testing.T) {
			runNoReplayOnRPCErrorCase(t, code)
		})
	}
}

type noReplayFixture struct {
	server *failingAcknowledgementServer
	peer   *failingAcknowledgementServer
	cm     *reporter.ConnectionManager
	conn   *grpc.ClientConn
	r      *gRPCReporter
}

func setupNoReplayHarness(t *testing.T, code codes.Code) noReplayFixture {
	t.Helper()
	server := &failingAcknowledgementServer{code: code}
	peer := &failingAcknowledgementServer{code: code}
	backends := serveFailingAcknowledgements(t, server) + "," + serveFailingAcknowledgements(t, peer)

	oldTimeout := reporter.BoundSendTimeoutForTest()
	reporter.SetBoundSendTimeoutForTest(400 * time.Millisecond)
	t.Cleanup(func() { reporter.SetBoundSendTimeoutForTest(oldTimeout) })

	auth := ""
	if code == codes.Unauthenticated || code == codes.PermissionDenied {
		auth = "token"
	}
	logger := &capturingLogger{}
	cm, err := reporter.NewConnectionManager(logger, time.Second, backends, auth, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cm.Close)
	conn, err := cm.GetConnection(backends)
	if err != nil {
		t.Fatal(err)
	}
	if _, holdErr := cm.GetConnection(backends); holdErr != nil {
		t.Fatal(holdErr)
	}
	cds, cdsErr := reporter.NewCDSManager(logger, backends, 0, cm)
	if cdsErr != nil {
		t.Fatal(cdsErr)
	}
	pprof, pprofErr := reporter.NewPprofTaskManager(logger, backends, time.Hour, cm, t.TempDir())
	if pprofErr != nil {
		t.Fatal(pprofErr)
	}
	rep, repErr := NewGRPCReporter(logger, backends, time.Second, time.Hour, cm, cds, pprof)
	if repErr != nil {
		t.Fatal(repErr)
	}
	r := rep.(*gRPCReporter)
	entity := &reporter.Entity{ServiceName: "policy", ServiceInstanceName: "inst"}
	r.entity = entity
	r.transform = reporter.NewTransform(entity)
	r.initSendPipeline()
	t.Cleanup(func() { r.Close() })
	return noReplayFixture{server: server, peer: peer, cm: cm, conn: conn, r: r}
}

func runNoReplayOnRPCErrorCase(t *testing.T, code codes.Code) {
	t.Helper()
	f := setupNoReplayHarness(t, code)
	segCount := func() int32 { return f.server.count.Load() + f.peer.count.Load() }
	segCalls := func() int32 { return f.server.calls.Load() + f.peer.calls.Load() }

	f.r.tracingSendCh <- &agentv3.SegmentObject{TraceId: "first", TraceSegmentId: "first"}
	waitUntil(t, 5*time.Second, func() bool { return segCount() >= 1 })
	if segCount() != 1 {
		t.Fatalf("first segment not received")
	}

	// Next Send observes the stream RPC error, discards this probe, reopens Collect.
	f.r.tracingSendCh <- &agentv3.SegmentObject{TraceId: "probe", TraceSegmentId: "probe"}
	waitUntil(t, 5*time.Second, func() bool { return segCalls() >= 2 })
	if segCount() != 1 {
		t.Fatalf("collectors received %d segments, want 1 (no replay after RPC error)", segCount())
	}
	if segCalls() < 2 {
		t.Fatalf("Collect opened %d times, want >=2 after RPC error reconnect", segCalls())
	}
	if f.r.conn != f.conn {
		t.Fatal("RPC path replaced the shared channel")
	}

	f.r.tracingSendCh <- &agentv3.SegmentObject{TraceId: "second", TraceSegmentId: "second"}
	waitUntil(t, 5*time.Second, func() bool { return segCount() >= 2 })
	if segCount() != 2 {
		t.Fatalf("after reconnect collectors received %d, want 2", segCount())
	}
}

func waitUntil(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestMultiBackendPipelineRecoversSendPanic(t *testing.T) {
	logger := &capturingLogger{}
	cm, err := reporter.NewConnectionManager(logger, time.Second, "127.0.0.1:1,127.0.0.1:2", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cm.Close)
	r := &gRPCReporter{logger: logger, connManager: cm}
	watchdog := reporter.NewBoundSendWatchdog(nil, 0)
	recovered, err := r.pipelineSend(watchdog, func() error { panic("corrupt protobuf payload") })
	if !recovered || err != nil {
		t.Fatalf("panic result: recovered=%v, err=%v", recovered, err)
	}
	recovered, err = r.pipelineSend(watchdog, func() error { return nil })
	if recovered || err != nil {
		t.Fatalf("subsequent send failed: recovered=%v, err=%v", recovered, err)
	}
}

func TestMultiBackendRetainsQueuedTracesWhileDisconnected(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	backends := lis.Addr().String() + ",127.0.0.1:1"
	cm, err := reporter.NewConnectionManager(&capturingLogger{}, 50*time.Millisecond, backends, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cm.Close()
	if _, getErr := cm.GetConnection(backends); getErr != nil {
		t.Fatal(getErr)
	}
	cds, cdsErr := reporter.NewCDSManager(&capturingLogger{}, backends, 0, cm)
	if cdsErr != nil {
		t.Fatal(cdsErr)
	}
	pprof, pprofErr := reporter.NewPprofTaskManager(&capturingLogger{}, backends, time.Hour, cm, t.TempDir())
	if pprofErr != nil {
		t.Fatal(pprofErr)
	}
	rep, repErr := NewGRPCReporter(&capturingLogger{}, backends, 50*time.Millisecond, time.Hour, cm, cds, pprof)
	if repErr != nil {
		t.Fatal(repErr)
	}
	r := rep.(*gRPCReporter)
	r.entity = &reporter.Entity{ServiceName: "q", ServiceInstanceName: "i"}
	r.transform = reporter.NewTransform(r.entity)
	r.initSendPipeline()
	r.tracingSendCh <- &agentv3.SegmentObject{TraceSegmentId: "queued"}
	time.Sleep(150 * time.Millisecond)
	if got := len(r.tracingSendCh); got != 1 {
		t.Fatalf("queue has %d traces, want the unsent trace retained while disconnected", got)
	}
	r.Close()
}

type recordingManagementClient struct {
	managementv3.ManagementServiceClient
	properties      atomic.Int32
	heartbeats      atomic.Int32
	propertiesError error
}

func (c *recordingManagementClient) ReportInstanceProperties(context.Context,
	*managementv3.InstanceProperties, ...grpc.CallOption) (*commonv3.Commands, error) {
	c.properties.Add(1)
	return &commonv3.Commands{}, c.propertiesError
}

func (c *recordingManagementClient) KeepAlive(context.Context,
	*managementv3.InstancePingPkg, ...grpc.CallOption) (*commonv3.Commands, error) {
	c.heartbeats.Add(1)
	return &commonv3.Commands{}, nil
}

// Unified path refreshes instance properties periodically and keeps heartbeats
// moving even when ReportInstanceProperties fails.
func TestMultiBackendRefreshesPropertiesWithoutBlockingHeartbeat(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "periodic refresh"
		if fail {
			name = "properties failure"
		}
		t.Run(name, func(t *testing.T) {
			a, b := serveBackendMocks(t), serveBackendMocks(t)
			defer a.stop()
			defer b.stop()
			backends := a.addr() + "," + b.addr()
			cm, err := reporter.NewConnectionManager(&capturingLogger{}, time.Second, backends, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cm.Close()
			if _, err := cm.GetConnection(backends); err != nil {
				t.Fatal(err)
			}
			client := &recordingManagementClient{}
			if fail {
				client.propertiesError = status.Error(codes.Unavailable, "retry later")
			}
			r := &gRPCReporter{
				logger:           &capturingLogger{},
				serverAddr:       backends,
				connManager:      cm,
				entity:           &reporter.Entity{},
				checkInterval:    10 * time.Millisecond,
				managementClient: client,
			}
			r.check()
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if client.properties.Load() >= 2 && client.heartbeats.Load() >= 10 {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatalf("properties=%d heartbeats=%d, want periodic properties and continuing heartbeats",
				client.properties.Load(), client.heartbeats.Load())
		})
	}
}
