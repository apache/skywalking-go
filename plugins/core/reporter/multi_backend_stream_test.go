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

package reporter

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	v3 "github.com/apache/skywalking-go/protocols/collect/common/v3"
	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
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

func TestMultiBackendCollectSendFailsOverAfterActiveStops(t *testing.T) {
	aSrv := &countingTraceServer{}
	bSrv := &countingTraceServer{}

	aLis, aGS := serveTrace(t, aSrv)
	defer aLis.Close()
	defer aGS.Stop()
	aAddr := aLis.Addr().String()
	bLis, bGS := serveTrace(t, bSrv)
	defer bLis.Close()
	defer bGS.Stop()
	bAddr := bLis.Addr().String()

	backends := aAddr + "," + bAddr
	cm, err := NewConnectionManager(nil, time.Second, backends, "", nil)
	if err != nil {
		t.Fatalf("NewConnectionManager: %v", err)
	}
	defer cm.Close()

	conn, err := cm.GetConnection(backends)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	client := agentv3.NewTraceSegmentReportServiceClient(conn)

	ctx, cancel, stopOpen := BackendStreamContext(time.Second)
	stream, err := client.Collect(metadata.NewOutgoingContext(ctx, cm.GetMD()))
	if timedOut := stopOpen(); err != nil || timedOut {
		cancel()
		t.Fatalf("open Collect: err=%v timedOut=%v", err, timedOut)
	}

	seg := &agentv3.SegmentObject{TraceId: "t1", TraceSegmentId: "s1"}
	if sendErr := BoundSend(cancel, func() error { return stream.Send(seg) }, time.Second); sendErr != nil {
		t.Fatalf("initial Send: %v", sendErr)
	}
	waitFor(t, func() bool { return aSrv.count.Load()+bSrv.count.Load() >= 1 }, 5*time.Second)

	// Stop active like docker kill: close listener hard so the peer half-opens.
	standby := bSrv
	if aSrv.count.Load() > 0 {
		aGS.Stop()
		_ = aLis.Close()
	} else {
		bGS.Stop()
		_ = bLis.Close()
		standby = aSrv
	}

	// Bound Send: either errors quickly or is canceled by BoundSend.
	_ = BoundSend(cancel, func() error {
		return stream.Send(&agentv3.SegmentObject{TraceId: "t2", TraceSegmentId: "s2"})
	}, 2*time.Second)
	cancel()

	// Native pick_first recovers on the same channel and generated stub.
	before := standby.count.Load()
	deadline := time.Now().Add(15 * time.Second)
	for attempt := 0; time.Now().Before(deadline); attempt++ {
		// The first RPC after Stop can race transport failure detection. Drop
		// that failed report and probe with a new ID, just as the reporter does.
		probeID := "post-stop-" + strconv.Itoa(attempt)
		if sendTraceProbe(client, cm.GetMD(), probeID) == nil && standby.count.Load() > before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("new reports did not reach the standby on the existing channel")
}

func sendTraceProbe(client agentv3.TraceSegmentReportServiceClient, md metadata.MD, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream, err := client.Collect(metadata.NewOutgoingContext(ctx, md), grpc.WaitForReady(true))
	if err != nil {
		return err
	}
	if sendErr := stream.Send(&agentv3.SegmentObject{TraceId: id, TraceSegmentId: id}); sendErr != nil {
		return sendErr
	}
	_, err = stream.CloseAndRecv()
	return err
}

func serveTrace(t *testing.T, srv agentv3.TraceSegmentReportServiceServer) (net.Listener, *grpc.Server) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	agentv3.RegisterTraceSegmentReportServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	return lis, gs
}

// peekConnection reads the managed ClientConn without changing refCount (test helper).
func peekConnection(cm *ConnectionManager, serverAddr string) *grpc.ClientConn {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	managed, exists := cm.connManager[serverAddr]
	if !exists {
		return nil
	}
	return managed.connection
}

// TestMultiBackendPerAddrDialTimeoutFailsOverPastBlackhole proves a first address
// that accepts TCP but never completes the gRPC handshake cannot starve the
// standby: each dial is capped by multiBackendPerAddrDialTimeout.
func TestMultiBackendPerAddrDialTimeoutFailsOverPastBlackhole(t *testing.T) {
	blackhole, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("blackhole listen: %v", err)
	}
	defer blackhole.Close()
	// Never Accept — TCP connects into the backlog, then HTTP/2 handshake hangs.

	healthySrv := &countingTraceServer{}
	healthyLis, healthyGS := serveTrace(t, healthySrv)
	defer healthyLis.Close()
	defer healthyGS.Stop()

	blackholeAddr := blackhole.Addr().String()
	healthyAddr := healthyLis.Addr().String()

	var cm *ConnectionManager
	var conn *grpc.ClientConn
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		backends := blackholeAddr + "," + healthyAddr
		var cmErr error
		cm, cmErr = NewConnectionManager(nil, time.Second, backends, "", nil)
		if cmErr != nil {
			t.Fatalf("NewConnectionManager: %v", cmErr)
		}
		conn, cmErr = cm.GetConnection(backends)
		if cmErr != nil {
			cm.Close()
			t.Fatalf("GetConnection: %v", cmErr)
		}
		// Wait briefly for the resolver to publish the shuffled order.
		publishDeadline := time.Now().Add(3 * time.Second)
		var resolved []string
		for time.Now().Before(publishDeadline) {
			resolved = resolvedBackendAddressesForTest(cm)
			if len(resolved) == 2 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if len(resolved) == 2 && resolved[0] == blackholeAddr {
			break
		}
		cm.Close()
		cm = nil
		conn = nil
	}
	if cm == nil || conn == nil {
		t.Fatal("could not obtain a channel with blackhole as first resolved address")
	}
	defer cm.Close()

	waitFor(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 15*time.Second)

	client := agentv3.NewTraceSegmentReportServiceClient(conn)
	if sendErr := sendTraceProbe(client, cm.GetMD(), "blackhole-failover"); sendErr != nil {
		t.Fatalf("probe after blackhole failover: %v", sendErr)
	}
	waitFor(t, func() bool { return healthySrv.count.Load() >= 1 }, 5*time.Second)
}

// TestMultiBackendPerAddrDialTimeoutFailsOverPastTLSSilentPeer proves a first
// address that finishes TLS but never speaks HTTP/2 cannot starve the standby.
// Clearing the dialer deadline on the first raw Read would pass here and hang.
func TestMultiBackendPerAddrDialTimeoutFailsOverPastTLSSilentPeer(t *testing.T) {
	serverTLS, clientCreds := testTLSMaterial(t)

	silentLis := serveSilentTLS(t, serverTLS)
	defer silentLis.Close()

	healthySrv := &countingTraceServer{}
	healthyLis, healthyGS := serveTraceTLS(t, healthySrv, serverTLS)
	defer healthyLis.Close()
	defer healthyGS.Stop()

	silentAddr := silentLis.Addr().String()
	healthyAddr := healthyLis.Addr().String()

	var cm *ConnectionManager
	var conn *grpc.ClientConn
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		backends := silentAddr + "," + healthyAddr
		var cmErr error
		cm, cmErr = NewConnectionManager(nil, time.Second, backends, "", clientCreds)
		if cmErr != nil {
			t.Fatalf("NewConnectionManager: %v", cmErr)
		}
		conn, cmErr = cm.GetConnection(backends)
		if cmErr != nil {
			cm.Close()
			t.Fatalf("GetConnection: %v", cmErr)
		}
		publishDeadline := time.Now().Add(3 * time.Second)
		var resolved []string
		for time.Now().Before(publishDeadline) {
			resolved = resolvedBackendAddressesForTest(cm)
			if len(resolved) == 2 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if len(resolved) == 2 && resolved[0] == silentAddr {
			break
		}
		cm.Close()
		cm = nil
		conn = nil
	}
	if cm == nil || conn == nil {
		t.Fatal("could not obtain a channel with TLS-silent peer as first resolved address")
	}
	defer cm.Close()

	waitFor(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 15*time.Second)

	client := agentv3.NewTraceSegmentReportServiceClient(conn)
	if sendErr := sendTraceProbe(client, cm.GetMD(), "tls-silent-failover"); sendErr != nil {
		t.Fatalf("probe after TLS-silent failover: %v", sendErr)
	}
	waitFor(t, func() bool { return healthySrv.count.Load() >= 1 }, 5*time.Second)
}

// TestCollectStreamSurvivesIdleAfterGOAWAY ensures graceful GOAWAY
// (channel Idle, accepted streams still draining) does not cancel the stream.
func TestCollectStreamSurvivesIdleAfterGOAWAY(t *testing.T) {
	srv := &countingTraceServer{}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()
	gs := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionAge:      300 * time.Millisecond,
		MaxConnectionAgeGrace: 5 * time.Second,
	}))
	agentv3.RegisterTraceSegmentReportServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()

	addr := lis.Addr().String()
	cm, err := NewConnectionManager(nil, time.Second, addr, "", nil)
	if err != nil {
		t.Fatalf("NewConnectionManager: %v", err)
	}
	defer cm.Close()
	conn, err := cm.GetConnection(addr)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	client := agentv3.NewTraceSegmentReportServiceClient(conn)

	ctx, cancel, stopOpen := BackendStreamContext(time.Second)
	defer cancel()
	stream, err := client.Collect(metadata.NewOutgoingContext(ctx, cm.GetMD()))
	if timedOut := stopOpen(); err != nil || timedOut {
		t.Fatalf("open Collect: err=%v timedOut=%v", err, timedOut)
	}

	if sendErr := stream.Send(&agentv3.SegmentObject{TraceId: "goaway-1", TraceSegmentId: "s1"}); sendErr != nil {
		t.Fatalf("first Send: %v", sendErr)
	}
	waitFor(t, func() bool { return conn.GetState() == connectivity.Idle }, 5*time.Second)

	select {
	case <-ctx.Done():
		t.Fatal("stream canceled on Idle during GOAWAY drain")
	default:
	}
	if sendErr := stream.Send(&agentv3.SegmentObject{TraceId: "goaway-2", TraceSegmentId: "s2"}); sendErr != nil {
		t.Fatalf("second Send during Idle drain: %v", sendErr)
	}
	if _, closeErr := stream.CloseAndRecv(); closeErr != nil {
		t.Fatalf("CloseAndRecv during Idle drain: %v", closeErr)
	}
}

func testTLSMaterial(t *testing.T) (*tls.Config, credentials.TransportCredentials) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	leaf, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  key,
		}},
		MinVersion: tls.VersionTLS12,
	}
	clientCreds := credentials.NewTLS(&tls.Config{
		RootCAs:    pool,
		ServerName: "localhost",
		MinVersion: tls.VersionTLS12,
	})
	return serverTLS, clientCreds
}

func serveSilentTLS(t *testing.T, serverTLS *tls.Config) net.Listener {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("silent TLS listen: %v", err)
	}
	go func() {
		for {
			raw, acceptErr := lis.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				tc := tls.Server(c, serverTLS.Clone())
				if hsErr := tc.Handshake(); hsErr != nil {
					_ = c.Close()
					return
				}
				// Stay silent after TLS — never speak HTTP/2 SETTINGS.
				buf := make([]byte, 1)
				for {
					_ = tc.SetReadDeadline(time.Now().Add(time.Hour))
					if _, readErr := tc.Read(buf); readErr != nil {
						_ = tc.Close()
						return
					}
				}
			}(raw)
		}
	}()
	return lis
}

func serveTraceTLS(t *testing.T, srv agentv3.TraceSegmentReportServiceServer, serverTLS *tls.Config) (net.Listener, *grpc.Server) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLS)))
	agentv3.RegisterTraceSegmentReportServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	return lis, gs
}

func TestBoundSendCancelsOnTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := BoundSend(cancel, func() error {
		<-ctx.Done()
		return ctx.Err()
	}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout cancel error")
	}
}

func TestBoundSendWatchdogInvokesCancel(t *testing.T) {
	// Watchdog cancel is what unblocks real gRPC SendMsg; verify it fires.
	canceled := make(chan struct{})
	cancel := func() {
		select {
		case <-canceled:
		default:
			close(canceled)
		}
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- BoundSend(cancel, func() error {
			select {
			case <-canceled:
				return context.Canceled
			case <-time.After(5 * time.Second):
				return nil
			}
		}, 50*time.Millisecond)
	}()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected canceled send error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("BoundSend did not return after watchdog cancel")
	}
}

func TestMultiBackendStatusReflectsConnClose(t *testing.T) {
	aLis, aGS := serveTrace(t, &countingTraceServer{})
	defer aLis.Close()
	defer aGS.Stop()
	bLis, bGS := serveTrace(t, &countingTraceServer{})
	defer bLis.Close()
	defer bGS.Stop()

	backends := aLis.Addr().String() + "," + bLis.Addr().String()
	cm, err := NewConnectionManager(nil, 50*time.Millisecond, backends, "", nil)
	if err != nil {
		t.Fatalf("NewConnectionManager: %v", err)
	}
	defer cm.Close()
	conn, err := cm.GetConnection(backends)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	_ = conn.Close()
	if got := cm.GetConnectionStatus(backends); got != ConnectionStatusShutdown {
		t.Fatalf("status=%v want Shutdown", got)
	}
}

func TestBoundSendPanicReachesCaller(t *testing.T) {
	for _, afterCancel := range []bool{false, true} {
		name := "immediate"
		if afterCancel {
			name = "after timeout cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var recovered interface{}
			func() {
				defer func() { recovered = recover() }()
				_ = BoundSend(cancel, func() error {
					if afterCancel {
						<-ctx.Done()
					}
					panic("protobuf marshal panic")
				}, 20*time.Millisecond)
			}()
			if recovered != "protobuf marshal panic" {
				t.Fatalf("caller recovered %v, want original panic", recovered)
			}
			if err := BoundSend(cancel, func() error { return nil }, time.Second); err != nil {
				t.Fatalf("next send failed: %v", err)
			}
		})
	}
}

// Concurrent acquisition and release must preserve shared connection ownership.
func TestConcurrentGetRelease(t *testing.T) {
	aLis, aGS := serveTrace(t, &countingTraceServer{})
	defer aLis.Close()
	defer aGS.Stop()
	bLis, bGS := serveTrace(t, &countingTraceServer{})
	defer bLis.Close()
	defer bGS.Stop()

	backends := aLis.Addr().String() + "," + bLis.Addr().String()
	cm, err := NewConnectionManager(nil, 50*time.Millisecond, backends, "", nil)
	if err != nil {
		t.Fatalf("NewConnectionManager: %v", err)
	}
	defer cm.Close()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				conn, getErr := cm.GetConnection(backends)
				if getErr != nil {
					t.Errorf("GetConnection: %v", getErr)
					return
				}
				if conn.GetState() == connectivity.Shutdown {
					t.Error("acquired connection closed while its reference is held")
				}
				_ = cm.ReleaseConnection(backends)
				_ = peekConnection(cm, backends)
				_ = cm.GetConnectionStatus(backends)
			}
		}()
	}
	wg.Wait()
}

func TestMultiBackendServiceConfigRetryOnlyProperties(t *testing.T) {
	var cfg struct {
		MethodConfig []struct {
			Name []struct {
				Service string `json:"service"`
				Method  string `json:"method"`
			} `json:"name"`
			RetryPolicy *struct {
				RetryableStatusCodes []string `json:"retryableStatusCodes"`
			} `json:"retryPolicy"`
		} `json:"methodConfig"`
	}
	if err := json.Unmarshal([]byte(multiBackendServiceConfig), &cfg); err != nil {
		t.Fatalf("service config JSON: %v", err)
	}
	var withRetry int
	for _, mc := range cfg.MethodConfig {
		if mc.RetryPolicy == nil {
			continue
		}
		withRetry++
		if len(mc.Name) != 1 ||
			mc.Name[0].Service != "skywalking.v3.ManagementService" ||
			mc.Name[0].Method != "reportInstanceProperties" {
			t.Fatalf("retryPolicy must only target reportInstanceProperties, got %+v", mc.Name)
		}
		if len(mc.RetryPolicy.RetryableStatusCodes) != 1 ||
			mc.RetryPolicy.RetryableStatusCodes[0] != "UNAVAILABLE" {
			t.Fatalf("unexpected retryableStatusCodes: %+v", mc.RetryPolicy.RetryableStatusCodes)
		}
	}
	if withRetry != 1 {
		t.Fatalf("want exactly 1 methodConfig with retryPolicy, got %d", withRetry)
	}
}

// TestRaceConnManagerGetRelease is picked up by `make test-race` (-run '^TestRace').
func TestRaceConnManagerGetRelease(t *testing.T) {
	TestConcurrentGetRelease(t)
}
