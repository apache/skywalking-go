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
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	v3 "github.com/apache/skywalking-go/protocols/collect/common/v3"
	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
	logv3 "github.com/apache/skywalking-go/protocols/collect/logging/v3"
)

type countingTraceServer struct {
	agentv3.UnimplementedTraceSegmentReportServiceServer
	count atomic.Int64
}

func (s *countingTraceServer) Collect(stream agentv3.TraceSegmentReportService_CollectServer) error {
	for {
		_, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&v3.Commands{})
			}
			return err
		}
		s.count.Add(1)
	}
}

type countingMeterServer struct {
	agentv3.UnimplementedMeterReportServiceServer
	count atomic.Int64
}

func (s *countingMeterServer) CollectBatch(stream agentv3.MeterReportService_CollectBatchServer) error {
	for {
		_, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&v3.Commands{})
			}
			return err
		}
		s.count.Add(1)
	}
}

type countingLogServer struct {
	logv3.UnimplementedLogReportServiceServer
	count atomic.Int64
}

func (s *countingLogServer) Collect(stream logv3.LogReportService_CollectServer) error {
	for {
		_, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&v3.Commands{})
			}
			return err
		}
		s.count.Add(1)
	}
}

type backendMocks struct {
	trace *countingTraceServer
	meter *countingMeterServer
	log   *countingLogServer
	lis   net.Listener
	gs    *grpc.Server
}

func serveBackendMocks(t *testing.T) *backendMocks {
	t.Helper()
	m := &backendMocks{
		trace: &countingTraceServer{},
		meter: &countingMeterServer{},
		log:   &countingLogServer{},
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	agentv3.RegisterTraceSegmentReportServiceServer(gs, m.trace)
	agentv3.RegisterMeterReportServiceServer(gs, m.meter)
	logv3.RegisterLogReportServiceServer(gs, m.log)
	m.lis = lis
	m.gs = gs
	go func() { _ = gs.Serve(lis) }()
	return m
}

func (m *backendMocks) stop() {
	_ = m.lis.Close()
	m.gs.Stop()
}

func (m *backendMocks) addr() string { return m.lis.Addr().String() }

func setupMultiReporter(t *testing.T, backends string) *gRPCReporter {
	t.Helper()
	oldTimeout := reporter.BoundSendTimeoutForTest()
	reporter.SetBoundSendTimeoutForTest(400 * time.Millisecond)
	t.Cleanup(func() {
		reporter.SetBoundSendTimeoutForTest(oldTimeout)
	})

	logger := &capturingLogger{}
	cm, err := reporter.NewConnectionManager(logger, time.Second, backends, "", nil)
	if err != nil {
		t.Fatalf("NewConnectionManager: %v", err)
	}
	t.Cleanup(func() { cm.Close() })

	cds, err := reporter.NewCDSManager(logger, backends, 0, cm)
	if err != nil {
		t.Fatalf("NewCDSManager: %v", err)
	}
	pprof, err := reporter.NewPprofTaskManager(logger, backends, time.Hour, cm, t.TempDir())
	if err != nil {
		t.Fatalf("NewPprofTaskManager: %v", err)
	}
	rep, err := NewGRPCReporter(logger, backends, time.Second, time.Hour, cm, cds, pprof)
	if err != nil {
		t.Fatalf("NewGRPCReporter: %v", err)
	}
	gr := rep.(*gRPCReporter)
	entity := &reporter.Entity{ServiceName: "auto-failover", ServiceInstanceName: "inst"}
	gr.entity = entity
	gr.transform = reporter.NewTransform(entity)
	gr.initSendPipeline()
	gr.bootFlag = true
	// Close the reporter before restoring BoundSend timeouts so pipeline
	// goroutines are not still reading those atomics during Cleanup.
	t.Cleanup(func() { gr.Close() })
	return gr
}

func waitTraceActive(t *testing.T, a, b *backendMocks) (active, standby *backendMocks) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a.trace.count.Load() > 0 {
			return a, b
		}
		if b.trace.count.Load() > 0 {
			return b, a
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("warm segment never reached either collector")
	return nil, nil
}

func waitCountGrow(t *testing.T, name string, before int64, get func() int64) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if get() > before {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s standby count did not grow: before=%d now=%d", name, before, get())
}

// TestMultiBackendReporterAutoFailsOverAfterActiveStops drives the real
// gRPCReporter send loop. Native pick_first moves reporting to the standby
// after the active mock stops, without replacing the shared ClientConn.
func TestMultiBackendReporterAutoFailsOverAfterActiveStops(t *testing.T) {
	a := serveBackendMocks(t)
	b := serveBackendMocks(t)
	defer a.stop()
	defer b.stop()
	backends := a.addr() + "," + b.addr()
	gr := setupMultiReporter(t, backends)

	seg := func(id string) *agentv3.SegmentObject {
		return &agentv3.SegmentObject{
			TraceId:         "t-" + id,
			TraceSegmentId:  "s-" + id,
			Service:         gr.entity.ServiceName,
			ServiceInstance: gr.entity.ServiceInstanceName,
		}
	}
	gr.tracingSendCh <- seg("warm")
	active, standby := waitTraceActive(t, a, b)
	before := standby.trace.count.Load()
	active.stop()

	for i := 0; i < 8; i++ {
		select {
		case gr.tracingSendCh <- seg("post"):
		default:
		}
		time.Sleep(200 * time.Millisecond)
		if standby.trace.count.Load() > before {
			return
		}
	}
	t.Fatalf("standby trace count did not grow: standby=%d before=%d active=%d",
		standby.trace.count.Load(), before, active.trace.count.Load())
}

// TestMultiBackendReporterMetricsAndLogFailOver covers CollectBatch / log Collect
// reconnect after active stop.
func TestMultiBackendReporterMetricsAndLogFailOver(t *testing.T) {
	a := serveBackendMocks(t)
	b := serveBackendMocks(t)
	defer a.stop()
	defer b.stop()
	backends := a.addr() + "," + b.addr()
	gr := setupMultiReporter(t, backends)

	seg := &agentv3.SegmentObject{
		TraceId: "t-warm", TraceSegmentId: "s-warm",
		Service: gr.entity.ServiceName, ServiceInstance: gr.entity.ServiceInstanceName,
	}
	gr.tracingSendCh <- seg
	active, standby := waitTraceActive(t, a, b)

	meters := []*agentv3.MeterData{{Service: gr.entity.ServiceName, ServiceInstance: gr.entity.ServiceInstanceName}}
	logMsg := &logv3.LogData{Timestamp: time.Now().UnixMilli(), Service: gr.entity.ServiceName, ServiceInstance: gr.entity.ServiceInstanceName}

	// Warm meter/log onto the same active channel (long-lived streams).
	select {
	case gr.metricsSendCh <- meters:
	case <-time.After(2 * time.Second):
		t.Fatal("metricsSendCh blocked")
	}
	select {
	case gr.logSendCh <- logMsg:
	case <-time.After(2 * time.Second):
		t.Fatal("logSendCh blocked")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (active.meter.count.Load() == 0 || active.log.count.Load() == 0) {
		time.Sleep(20 * time.Millisecond)
	}
	if active.meter.count.Load() == 0 || active.log.count.Load() == 0 {
		t.Fatalf("warm meter/log missing: meter=%d log=%d", active.meter.count.Load(), active.log.count.Load())
	}

	beforeMeter := standby.meter.count.Load()
	beforeLog := standby.log.count.Load()
	active.stop()

	for i := 0; i < 12; i++ {
		select {
		case gr.metricsSendCh <- meters:
		default:
		}
		select {
		case gr.logSendCh <- logMsg:
		default:
		}
		select {
		case gr.tracingSendCh <- seg:
		default:
		}
		time.Sleep(200 * time.Millisecond)
	}
	waitCountGrow(t, "meter", beforeMeter, standby.meter.count.Load)
	waitCountGrow(t, "log", beforeLog, standby.log.count.Load)
}
