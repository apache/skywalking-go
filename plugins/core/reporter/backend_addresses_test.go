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
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

const (
	testBackendHost     = "oap.example.com"
	testBackendAddr     = testBackendHost + ":11800"
	testIPLiteralAddr   = "10.0.0.1:11800"
	testBackendHostA    = "a.example.com"
	testBackendHostB    = "b.example.com"
	testBackendAddrA    = testBackendHostA + ":11800"
	testBackendAddrB    = testBackendHostB + ":11800"
	testMultiBackendCSV = testBackendAddrA + "," + testBackendAddrB
)

type testResolverClientConn struct {
	mu    sync.Mutex
	state resolver.State
	count int
}

func (c *testResolverClientConn) UpdateState(s resolver.State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = s
	c.count++
	return nil
}
func (c *testResolverClientConn) ReportError(error)             {}
func (c *testResolverClientConn) NewAddress([]resolver.Address) {}
func (c *testResolverClientConn) NewServiceConfig(string)       {}
func (c *testResolverClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return &serviceconfig.ParseResult{}
}

func (c *testResolverClientConn) lastState() resolver.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func hasExactAddresses(state resolver.State, want []string) bool {
	if len(state.Addresses) != len(want) {
		return false
	}
	got := make([]string, len(state.Addresses))
	for i, addr := range state.Addresses {
		got[i] = addr.Addr
	}
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	return reflect.DeepEqual(got, want)
}

func TestBackendResolverServerNamePerAddress(t *testing.T) {
	cases := map[string][]string{
		"hostnames": {testBackendAddrA, testBackendAddrB},
		"mixed":     {testIPLiteralAddr, testBackendAddr},
		"ipv4":      {testIPLiteralAddr, "10.0.0.2:11800"},
		"ipv6":      {"[2001:db8::1]:11800", "[2001:db8::2]:11800"},
	}
	for name, backends := range cases {
		t.Run(name, func(t *testing.T) {
			addrs := configuredAddressesAsResolverState(backends)
			if !hasExactAddresses(resolver.State{Addresses: addrs}, backends) {
				t.Fatalf("addresses=%+v", addrs)
			}
			for i, addr := range addrs {
				host, _, err := net.SplitHostPort(backends[i])
				if err != nil {
					t.Fatalf("split %q: %v", backends[i], err)
				}
				if addr.ServerName != host {
					t.Fatalf("endpoint %q ServerName=%q, want %q", addr.Addr, addr.ServerName, host)
				}
			}
		})
	}
}

func TestStaticMultiBackendKeepsOrderAcrossResolveNow(t *testing.T) {
	var published []string
	raw := testMultiBackendCSV + "," + testIPLiteralAddr
	builder, err := newStaticBackendResolverBuilder(nil, strings.Split(raw, ","), func(addrs []string) { published = addrs })
	if err != nil {
		t.Fatal(err)
	}
	cc := &testResolverClientConn{}
	r, err := builder.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	initial := append([]resolver.Address(nil), cc.lastState().Addresses...)
	if !hasExactAddresses(cc.lastState(), []string{testBackendAddrA, testBackendAddrB, testIPLiteralAddr}) {
		t.Fatalf("shuffled addresses=%+v", initial)
	}
	if len(published) != len(initial) {
		t.Fatalf("published=%v", published)
	}
	for i, addr := range initial {
		if addr.Addr != published[i] {
			t.Fatalf("published order=%v, resolver order=%v", published, initial)
		}
	}
	for i := 0; i < 10; i++ {
		r.ResolveNow(resolver.ResolveNowOptions{})
		if !reflect.DeepEqual(cc.lastState().Addresses, initial) {
			t.Fatalf("ResolveNow changed order from %v to %v", initial, cc.lastState().Addresses)
		}
	}
	if got := builder.target(); got != staticBackendScheme+":///"+raw {
		t.Fatalf("shuffle changed target/authority: %q", got)
	}
}

func TestParseBackendServiceList(t *testing.T) {
	got, err := parseBackendServiceList(" "+testBackendAddrA+" , "+testIPLiteralAddr+", "+testBackendAddrA+" ", nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 || got[0] != testBackendAddrA || got[1] != testIPLiteralAddr {
		t.Fatalf("got %#v", got)
	}
	if _, emptyErr := parseBackendServiceList("", nil); emptyErr != errNoValidBackendService {
		t.Fatal("expected empty error")
	}
	passthrough, passthroughErr := parseBackendServiceList("bad", nil)
	if passthroughErr != nil || len(passthrough) != 1 || passthrough[0] != "bad" {
		t.Fatalf("single non-host:port must passthrough, got %#v err=%v", passthrough, passthroughErr)
	}
	v6, err := parseBackendServiceList("[2001:db8::1]:11800", nil)
	if err != nil {
		t.Fatalf("ipv6: %v", err)
	}
	if len(v6) != 1 || v6[0] != "[2001:db8::1]:11800" {
		t.Fatalf("ipv6 got %#v", v6)
	}
}

func TestParseBackendServiceListURIPassthrough(t *testing.T) {
	for _, uri := range []string{
		"dns:///oap.example:11800",
		"unix:///tmp/oap.sock",
		"dns:oap.example:11800",
	} {
		got, err := parseBackendServiceList(uri, nil)
		if err != nil || len(got) != 1 || got[0] != uri {
			t.Fatalf("%q: got %#v err=%v", uri, got, err)
		}
	}
	// Multi-address lists still skip invalid host:port entries.
	got, err := parseBackendServiceList("dns:///x,"+testBackendAddr, nil)
	if err != nil || !reflect.DeepEqual(got, []string{testBackendAddr}) {
		t.Fatalf("multi list must skip URI tokens: %#v err=%v", got, err)
	}
}

func TestIsIPLiteralHost(t *testing.T) {
	if !isIPLiteralHost("127.0.0.1") || !isIPLiteralHost("::1") || !isIPLiteralHost("[::1]") {
		t.Fatal("expected IP literals")
	}
	if isIPLiteralHost(testBackendHost) || isIPLiteralHost("") {
		t.Fatal("expected non-IP")
	}
}

func isIPLiteralHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	return net.ParseIP(host) != nil
}

func TestBackendServicePortValidation(t *testing.T) {
	for _, port := range []string{"0", "65536", "grpc", "1.5", "+11800", "-1"} {
		logger := &captureAuthLogger{}
		got, err := parseBackendServiceList("oap:"+port+","+testBackendAddr, logger)
		if err != nil || !reflect.DeepEqual(got, []string{testBackendAddr}) {
			t.Errorf("invalid port %q was not skipped: %v, %v", port, got, err)
		}
		if len(logger.warnings) != 1 || !strings.Contains(logger.warnings[0], "oap:"+port) || len(logger.errors) != 0 {
			t.Errorf("invalid port %q: warnings=%v errors=%v", port, logger.warnings, logger.errors)
		}
	}
	got, err := parseBackendServiceList("oap:011800,oap:11800", nil)
	if err != nil || len(got) != 1 || got[0] != "oap:11800" {
		t.Fatalf("port normalization: %v, %v", got, err)
	}
}

type recordingAuthorityCredentials struct {
	credentials.TransportCredentials
	serverName  string
	authorities chan string
}

func (c *recordingAuthorityCredentials) Info() credentials.ProtocolInfo {
	info := c.TransportCredentials.Info()
	info.ServerName = c.serverName
	return info
}

func (c *recordingAuthorityCredentials) Clone() credentials.TransportCredentials {
	return &recordingAuthorityCredentials{
		TransportCredentials: c.TransportCredentials.Clone(), serverName: c.serverName, authorities: c.authorities,
	}
}

func (c *recordingAuthorityCredentials) ClientHandshake(ctx context.Context, authority string,
	conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	c.authorities <- authority
	return c.TransportCredentials.ClientHandshake(ctx, authority, conn)
}

func TestMultiBackendChannelAuthority(t *testing.T) {
	a, aServer := serveTrace(t, &countingTraceServer{})
	defer a.Close()
	defer aServer.Stop()
	b, bServer := serveTrace(t, &countingTraceServer{})
	defer b.Close()
	defer bServer.Stop()
	backends := "invalid," + a.Addr().String() + "," + b.Addr().String()
	aHost, _, _ := net.SplitHostPort(a.Addr().String())
	bHost, _, _ := net.SplitHostPort(b.Addr().String())
	creds := &recordingAuthorityCredentials{
		TransportCredentials: insecure.NewCredentials(), authorities: make(chan string, 10),
	}
	logger := &captureAuthLogger{}
	cm, err := NewConnectionManager(logger, time.Second, backends, "", creds)
	if err != nil {
		t.Fatal(err)
	}
	defer cm.Close()
	if _, err := cm.GetConnection(backends); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-creds.authorities:
		// Per-address ServerName uses the endpoint host (IP literal here).
		if got != aHost && got != bHost {
			t.Fatalf("handshake authority=%q, want %q or %q", got, aHost, bHost)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transport handshake did not start")
	}
	if !cm.IsMultiBackend() {
		t.Fatal("two valid endpoints must use the multi-backend policy")
	}
	if len(logger.warnings) != 1 || !strings.Contains(logger.warnings[0], "invalid") || len(logger.errors) != 0 {
		t.Fatalf("warnings=%v errors=%v", logger.warnings, logger.errors)
	}
}

func TestStaticMultiBackendPublishesLiterals(t *testing.T) {
	builder, err := newStaticBackendResolverBuilder(nil, []string{testBackendAddrA, testBackendAddrB}, nil)
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	cc := &testResolverClientConn{}
	r, err := builder.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer r.Close()

	if !hasExactAddresses(cc.lastState(), []string{testBackendAddrA, testBackendAddrB}) {
		t.Fatalf("state=%+v", cc.lastState())
	}
}

func TestStaticMultiBackendDoesNotExpandDNS(t *testing.T) {
	builder, err := newStaticBackendResolverBuilder(nil, []string{testIPLiteralAddr, "10.0.0.2:11800"}, nil)
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	cc := &testResolverClientConn{}
	r, err := builder.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer r.Close()
	if !hasExactAddresses(cc.lastState(), []string{testIPLiteralAddr, "10.0.0.2:11800"}) {
		t.Fatalf("state=%+v", cc.lastState())
	}
}

func TestMultiBackendPickFirstFailsOver(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()
	goodAddr := lis.Addr().String()
	gs := grpc.NewServer()
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()

	badLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bad listen: %v", err)
	}
	badAddr := badLis.Addr().String()
	_ = badLis.Close()

	backends := []string{badAddr, goodAddr}
	builder, err := newStaticBackendResolverBuilder(nil, backends, nil)
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	conn, err := grpc.Dial(builder.target(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithResolvers(builder),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	waitFor(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 8*time.Second)
}

func TestConnectionManagerMultiBackendPickFirst(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()
	goodAddr := lis.Addr().String()
	gs := grpc.NewServer()
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()

	badLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bad listen: %v", err)
	}
	badAddr := badLis.Addr().String()
	_ = badLis.Close()

	backends := badAddr + "," + goodAddr
	cm, err := NewConnectionManager(nil, time.Second, backends, "", nil)
	if err != nil {
		t.Fatalf("NewConnectionManager: %v", err)
	}
	defer cm.Close()

	conn, err := cm.GetConnection(backends)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	waitFor(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 8*time.Second)
}

func TestConnectionManagerFailoverAfterActiveStops(t *testing.T) {
	aLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen a: %v", err)
	}
	aAddr := aLis.Addr().String()
	aGS := grpc.NewServer()
	go func() { _ = aGS.Serve(aLis) }()

	bLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen b: %v", err)
	}
	bAddr := bLis.Addr().String()
	bGS := grpc.NewServer()
	go func() { _ = bGS.Serve(bLis) }()
	t.Cleanup(func() {
		aGS.Stop()
		_ = aLis.Close()
		bGS.Stop()
		_ = bLis.Close()
	})

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
	waitFor(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 8*time.Second)

	// pick_first uses the shuffled resolver order; stop the active (first) peer.
	resolved := resolvedBackendAddressesForTest(cm)
	if len(resolved) == 0 {
		t.Fatal("expected published resolver addresses")
	}
	switch resolved[0] {
	case aAddr:
		aGS.Stop()
		_ = aLis.Close()
	case bAddr:
		bGS.Stop()
		_ = bLis.Close()
	default:
		t.Fatalf("unexpected active address %q", resolved[0])
	}

	waitFor(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 15*time.Second)
	if status := cm.GetConnectionStatus(backends); status == ConnectionStatusShutdown {
		t.Fatalf("status=%v after failover", status)
	}
}

func TestConnectionManagerNormalizedSingleBackendDial(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()
	addr := lis.Addr().String()
	gs := grpc.NewServer()
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()

	cases := []struct {
		raw      string
		warnings int
	}{
		{addr + "," + addr, 0}, // duplicate collapses to one
		{addr + ",", 0},        // trailing comma
		{"invalid," + addr, 1}, // invalid endpoint is skipped
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			logger := &captureAuthLogger{}
			cm, err := NewConnectionManager(logger, time.Second, tc.raw, "", nil)
			if err != nil {
				t.Fatalf("NewConnectionManager: %v", err)
			}
			defer cm.Close()
			conn, err := cm.GetConnection(tc.raw)
			if err != nil {
				t.Fatalf("GetConnection: %v", err)
			}
			waitFor(t, func() bool {
				return conn.GetState() == connectivity.Ready
			}, 8*time.Second)
			if cm.IsMultiBackend() || conn.Target() != addr {
				t.Fatalf("single surviving endpoint uses wrong policy: multi=%v target=%q", cm.IsMultiBackend(), conn.Target())
			}
			if _, err := cm.GetConnection(tc.raw); err != nil {
				t.Fatal(err)
			}
			if len(logger.warnings) != tc.warnings || len(logger.errors) != 0 {
				t.Fatalf("warnings=%v errors=%v", logger.warnings, logger.errors)
			}
		})
	}
}

func TestConnectionManagerNoValidBackends(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		warnings int
	}{
		{"bad,oap:0,[::1]:65536", 4},
		{" , , ", 1},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			logger := &captureAuthLogger{}
			cm, err := NewConnectionManager(logger, time.Second, tc.raw, "", nil)
			if cm != nil || err != errNoValidBackendService {
				t.Fatalf("no valid endpoints must disable reporting: manager=%v err=%v", cm, err)
			}
			if len(logger.warnings) != tc.warnings || len(logger.errors) != 0 {
				t.Fatalf("warnings=%v errors=%v", logger.warnings, logger.errors)
			}
			if !strings.Contains(logger.warnings[len(logger.warnings)-1], "no valid backend") {
				t.Fatalf("missing no-valid warning: %v", logger.warnings)
			}
		})
	}
}
