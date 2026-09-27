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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/connectivity"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

var authKey = "Authentication"

// multiBackendServiceConfig configures pick_first plus UNAVAILABLE retries only on
// unary reportInstanceProperties. Client-streaming Collect (trace/meter/log) must
// not retry — the write buffer can be replayed and OAP does not dedupe segments.
const multiBackendServiceConfig = `{
  "loadBalancingConfig": [{"pick_first":{}}],
  "methodConfig": [
    {
      "name": [{}],
      "waitForReady": false
    },
    {
      "name": [{"service": "skywalking.v3.ManagementService", "method": "reportInstanceProperties"}],
      "waitForReady": false,
      "retryPolicy": {
        "maxAttempts": 3,
        "initialBackoff": "1s",
        "maxBackoff": "10s",
        "backoffMultiplier": 2,
        "retryableStatusCodes": ["UNAVAILABLE"]
      }
    }
  ]
}`

const multiBackendDialTimeout = 5 * time.Second

func NewConnectionManager(logger operator.LogOperator, checkInterval time.Duration,
	serverAddr string, auth string, creds credentials.TransportCredentials) (*ConnectionManager, error) {
	c := &ConnectionManager{
		logger:        logger,
		checkInterval: checkInterval,
		serverAddr:    serverAddr,
		md:            metadata.New(map[string]string{authKey: auth}),
		creds:         creds,
		connManager:   make(map[string]*ManagedConnection),
		mu:            sync.RWMutex{},
	}
	// Normalize once so skipped-entry warnings are not repeated when channels
	// are acquired or the resolver refreshes.
	backends, err := parseBackendServiceList(serverAddr, logger)
	if err != nil {
		if logger != nil {
			logger.Warnf("%v", err)
		}
		return nil, err
	}
	c.backends = backends
	c.multiBackend = len(backends) >= 2
	if !c.multiBackend {
		c.serverAddr = backends[0]
	}
	// Auth-failure throttled logs are used on the multi-address dial path.
	if c.multiBackend {
		c.authFailures = &authFailureLogger{logger: logger}
	}
	return c, nil
}

type ConnectionManager struct {
	logger        operator.LogOperator
	checkInterval time.Duration
	serverAddr    string
	md            metadata.MD
	creds         credentials.TransportCredentials
	connManager   map[string]*ManagedConnection
	mu            sync.RWMutex
	backends      []string

	// multiBackend is true when config normalizes to ≥2 addresses (static resolver).
	multiBackend         bool
	resolvedMu           sync.RWMutex
	resolvedBackendAddrs []string
	authFailures         *authFailureLogger
}

type ManagedConnection struct {
	connection *grpc.ClientConn
	status     ConnectionStatus
	refCount   int
}

func (cm *ConnectionManager) GetMD() metadata.MD {
	return cm.md
}

// IsMultiBackend reports whether this manager was configured with two or more
// distinct backend addresses after normalize.
func (cm *ConnectionManager) IsMultiBackend() bool {
	return cm.multiBackend
}

func (cm *ConnectionManager) GetConnection(serverAddr string) (*grpc.ClientConn, error) {
	// Serialize reference counts and map access across acquisition and release.
	cm.mu.Lock()
	if managed, exists := cm.connManager[serverAddr]; exists {
		managed.refCount++
		conn := managed.connection
		cm.mu.Unlock()
		return conn, nil
	}
	cm.mu.Unlock()

	conn, err := cm.createConnection()
	if err != nil {
		return nil, err
	}

	cm.mu.Lock()
	if managed, exists := cm.connManager[serverAddr]; exists {
		managed.refCount++
		existing := managed.connection
		cm.mu.Unlock()
		_ = conn.Close()
		return existing, nil
	}
	cm.connManager[serverAddr] = &ManagedConnection{
		connection: conn,
		status:     ConnectionStatusConnected,
		refCount:   1,
	}
	cm.mu.Unlock()
	go cm.checkConnectionStatus(serverAddr)
	return conn, nil
}

func (cm *ConnectionManager) createConnection() (*grpc.ClientConn, error) {
	// Single-address options also apply when filtering leaves one endpoint.
	if !cm.multiBackend {
		var credsDialOption grpc.DialOption
		if cm.creds != nil {
			credsDialOption = grpc.WithTransportCredentials(cm.creds)
		} else {
			credsDialOption = grpc.WithTransportCredentials(insecure.NewCredentials())
		}

		conn, err := grpc.Dial(cm.serverAddr, credsDialOption, grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   cm.checkInterval,
			},
		}))
		return conn, err
	}

	return cm.dialMultiBackend(cm.backends)
}

func (cm *ConnectionManager) dialMultiBackend(backends []string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if cm.creds != nil {
		opts = append(opts, grpc.WithTransportCredentials(cm.creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	target, multiOpts, multiErr := cm.multiBackendDialOptions(backends)
	if multiErr != nil {
		return nil, multiErr
	}
	opts = append(opts, multiOpts...)
	opts = append(opts, grpc.WithConnectParams(grpc.ConnectParams{
		MinConnectTimeout: multiBackendDialTimeout,
		Backoff: backoff.Config{
			BaseDelay:  1.0 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   cm.checkInterval,
		},
	}))
	conn, dialErr := grpc.Dial(target, opts...)
	if dialErr != nil {
		if cm.logger != nil {
			cm.logger.Errorf("dial multi-backend target %q failed: %v", target, dialErr)
		}
		return nil, fmt.Errorf("dial backend %q via static multi-backend resolver: %w", target, dialErr)
	}
	return conn, nil
}

// multiBackendDialOptions configures the static pick_first resolver path used
// when backend_service lists two or more addresses.
func (cm *ConnectionManager) multiBackendDialOptions(backends []string) (string, []grpc.DialOption, error) {
	builder, buildErr := newStaticBackendResolverBuilder(cm.logger, backends, cm.storeResolvedBackendAddresses)
	if buildErr != nil {
		if cm.logger != nil {
			cm.logger.Errorf("create static multi-backend resolver for %q failed: %v",
				cm.serverAddr, buildErr)
		}
		return "", nil, fmt.Errorf("create static backend resolver: %w", buildErr)
	}
	opts := []grpc.DialOption{
		grpc.WithResolvers(builder),
		grpc.WithDefaultServiceConfig(multiBackendServiceConfig),
		// Bypass HTTP(S) proxy for multi-address channels.
		grpc.WithContextDialer(directTCPContextDialer),
	}
	if cm.authFailures != nil {
		opts = append(opts,
			grpc.WithUnaryInterceptor(cm.authFailures.unaryInterceptor()),
			grpc.WithStreamInterceptor(cm.authFailures.streamInterceptor()),
		)
	}
	if cm.logger != nil {
		cm.logger.Infof("using static multi-backend pick_first resolver (%d addresses): %s",
			len(backends), strings.Join(backends, ","))
	}
	return builder.target(), opts, nil
}

func (cm *ConnectionManager) storeResolvedBackendAddresses(addrs []string) {
	copied := append([]string(nil), addrs...)
	cm.resolvedMu.Lock()
	cm.resolvedBackendAddrs = copied
	cm.resolvedMu.Unlock()
}

// ResolvedBackendAddresses returns the last address list published to gRPC.
// Empty when the static multi-backend resolver is unused. Intended for tests
// and diagnostics.
func (cm *ConnectionManager) ResolvedBackendAddresses() []string {
	cm.resolvedMu.RLock()
	defer cm.resolvedMu.RUnlock()
	return append([]string(nil), cm.resolvedBackendAddrs...)
}

func directTCPContextDialer(ctx context.Context, addr string) (net.Conn, error) {
	// Bound dial so pick_first can leave a blackhole first address quickly.
	// TCP keepalive helps, but half-open peers can still look Ready for a long
	// time — BoundSend bounds stream Send so failover is not stuck.
	d := &net.Dialer{Timeout: multiBackendDialTimeout, KeepAlive: 10 * time.Second}
	return d.DialContext(ctx, "tcp", addr)
}

// boundSendTimeoutNs / boundSendCancelGraceNs are atomic so tests can shorten
// BoundSend without racing pipeline goroutines that still read the defaults.
var (
	boundSendTimeoutNs     atomic.Int64
	boundSendCancelGraceNs atomic.Int64
)

func init() {
	boundSendTimeoutNs.Store(int64(8 * time.Second))
	boundSendCancelGraceNs.Store(int64(2 * time.Second))
}

var errBoundSendTimeout = fmt.Errorf("bound send timed out")

func boundSendTimeout() time.Duration {
	return time.Duration(boundSendTimeoutNs.Load())
}

func boundSendCancelGrace() time.Duration {
	return time.Duration(boundSendCancelGraceNs.Load())
}

// BoundSendTimeoutForTest / SetBoundSendTimeoutForTest let unit tests exercise
// kill→standby without waiting the production 8s bound.
func BoundSendTimeoutForTest() time.Duration { return boundSendTimeout() }
func SetBoundSendTimeoutForTest(d time.Duration) {
	boundSendTimeoutNs.Store(int64(d))
}
func BoundSendCancelGraceForTest() time.Duration { return boundSendCancelGrace() }
func SetBoundSendCancelGraceForTest(d time.Duration) {
	boundSendCancelGraceNs.Store(int64(d))
}

// Aliases retained for existing tests.
func MultiBackendSendTimeoutForTest() time.Duration     { return BoundSendTimeoutForTest() }
func SetMultiBackendSendTimeoutForTest(d time.Duration) { SetBoundSendTimeoutForTest(d) }
func MultiBackendSendCancelGraceForTest() time.Duration {
	return BoundSendCancelGraceForTest()
}
func SetMultiBackendSendCancelGraceForTest(d time.Duration) {
	SetBoundSendCancelGraceForTest(d)
}

// BoundSend bounds a Collect Send/CloseAndRecv with a stream-context deadline:
// after timeout it invokes cancel (same effect as ctx deadline). Send still runs
// in one helper goroutine so a half-open peer that ignores cancel cannot block
// the reporter past timeout+grace.
//
// The buffered result lets the worker finish after the caller times out.
// Recover in the worker and re-panic in the caller so reporter recovery still
// protects the application from protobuf encoding panics.
func BoundSend(cancel context.CancelFunc, send func() error, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = boundSendTimeout()
	}
	type sendResult struct {
		err        error
		panicValue interface{}
	}
	done := make(chan sendResult, 1)
	go func() {
		result := sendResult{}
		defer func() {
			result.panicValue = recover()
			done <- result
		}()
		result.err = send()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-done:
		if result.panicValue != nil {
			panic(result.panicValue)
		}
		return result.err
	case <-timer.C:
		if cancel != nil {
			cancel()
		}
		grace := time.NewTimer(boundSendCancelGrace())
		defer grace.Stop()
		select {
		case result := <-done:
			if result.panicValue != nil {
				panic(result.panicValue)
			}
			if result.err != nil {
				return result.err
			}
			return errBoundSendTimeout
		case <-grace.C:
			return errBoundSendTimeout
		}
	}
}

// MultiBackendSend is an alias for BoundSend.
func MultiBackendSend(cancel context.CancelFunc, send func() error, timeout time.Duration) error {
	return BoundSend(cancel, send, timeout)
}

// BackendRPCContext returns the context for an outbound unary backend RPC.
func BackendRPCContext(serverAddr string, atLeast time.Duration) (context.Context, context.CancelFunc) {
	_ = serverAddr
	timeout := 30 * time.Second
	if atLeast > timeout {
		timeout = atLeast
	}
	return context.WithTimeout(context.Background(), timeout)
}

// BackendStreamContext is for long-lived Collect streams. Call stopOpenTimer
// immediately after Collect returns; if it reports timedOut, discard the stream.
// After a successful open, callers should start WatchConnCancelOnUnready so a
// hung Send unblocks when the channel leaves Ready.
func BackendStreamContext(serverAddr string, atLeast time.Duration) (
	ctx context.Context, cancel context.CancelFunc, stopOpenTimer func() (timedOut bool),
) {
	_ = serverAddr
	ctx, cancel = context.WithCancel(context.Background())
	timeout := 30 * time.Second
	if atLeast > timeout {
		timeout = atLeast
	}
	var mu sync.Mutex
	opened := false
	timer := time.AfterFunc(timeout, func() {
		mu.Lock()
		defer mu.Unlock()
		if !opened {
			cancel()
		}
	})
	stopOpenTimer = func() bool {
		mu.Lock()
		opened = true
		timedOut := ctx.Err() != nil
		mu.Unlock()
		timer.Stop()
		return timedOut
	}
	return ctx, cancel, stopOpenTimer
}

// WatchConnCancelOnUnready cancels ctx once conn has been Ready and later
// enters TransientFailure or Shutdown. Start only after Collect succeeds so
// pick_first can finish Connecting → Ready on the standby without being
// canceled early.
func WatchConnCancelOnUnready(ctx context.Context, cancel context.CancelFunc, conn *grpc.ClientConn) {
	if conn == nil {
		return
	}
	seenReady := false
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			seenReady = true
		}
		if seenReady && (state == connectivity.TransientFailure || state == connectivity.Shutdown) {
			cancel()
			return
		}
		if !conn.WaitForStateChange(ctx, state) {
			return
		}
	}
}

// PeekConnection returns the managed ClientConn for serverAddr without
// changing refCount. Nil when missing.
func (cm *ConnectionManager) PeekConnection(serverAddr string) *grpc.ClientConn {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	managed, exists := cm.connManager[serverAddr]
	if !exists {
		return nil
	}
	return managed.connection
}

func (cm *ConnectionManager) checkConnectionStatus(serverAddr string) {
	for {
		cm.mu.Lock()
		managed, exists := cm.connManager[serverAddr]
		if !exists {
			cm.mu.Unlock()
			return
		}
		conn := managed.connection
		multi := cm.multiBackend
		checkInterval := cm.checkInterval
		cm.mu.Unlock()

		state := conn.GetState()
		// Nudge idle/TF channels so pick_first can migrate off a dead backend.
		if multi && (state == connectivity.Idle || state == connectivity.TransientFailure) {
			conn.Connect()
		}
		var newStatus ConnectionStatus
		switch state {
		case connectivity.TransientFailure:
			newStatus = ConnectionStatusDisconnect
		case connectivity.Shutdown:
			newStatus = ConnectionStatusShutdown
		default:
			// Idle, Connecting, and Ready all report Connected so pipeline loops
			// keep trying Collect while the transport recovers.
			newStatus = ConnectionStatusConnected
		}
		cm.mu.Lock()
		current, stillExists := cm.connManager[serverAddr]
		// Entry gone or replaced: this watcher no longer owns the map slot.
		if !stillExists || current != managed {
			cm.mu.Unlock()
			return
		}
		if newStatus != current.status {
			current.status = newStatus
		}
		cm.mu.Unlock()
		interval := 5 * time.Second
		if multi && checkInterval > 0 {
			interval = checkInterval
		}
		time.Sleep(interval)
	}
}

func (cm *ConnectionManager) ReleaseConnection(serverAddr string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	managed, exists := cm.connManager[serverAddr]
	if !exists {
		return nil
	}
	managed.refCount--
	if managed.refCount <= 0 {
		if err := managed.connection.Close(); err != nil {
			cm.logger.Error(err)
		}
		delete(cm.connManager, serverAddr)
	}
	return nil
}

// Close force-closes every managed ClientConn. Test helper for multi-backend
// cases; production reporter shutdown uses ReleaseConnection.
func (cm *ConnectionManager) Close() {
	cm.mu.Lock()
	conns := make([]*grpc.ClientConn, 0, len(cm.connManager))
	for addr, managed := range cm.connManager {
		conns = append(conns, managed.connection)
		delete(cm.connManager, addr)
	}
	cm.mu.Unlock()
	for _, conn := range conns {
		if err := conn.Close(); err != nil && cm.logger != nil {
			cm.logger.Error(err)
		}
	}
}

func (cm *ConnectionManager) GetConnectionStatus(serverAddr string) ConnectionStatus {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	managed, exists := cm.connManager[serverAddr]
	if !exists {
		return ConnectionStatusShutdown
	}
	// Reflect a closed ClientConn immediately; the background checker may still
	// be sleeping on checkInterval before it writes Shutdown into managed.status.
	if managed.connection != nil && managed.connection.GetState() == connectivity.Shutdown {
		managed.status = ConnectionStatusShutdown
	}
	return managed.status
}

// nolint
func generateTLSCredential(caPath, clientKeyPath, clientCertChainPath string, skipVerify bool) (tc credentials.TransportCredentials, tlsErr error) {
	if err := checkTLSFile(caPath); err != nil {
		return nil, err
	}
	tlsConfig := new(tls.Config)
	tlsConfig.Renegotiation = tls.RenegotiateNever
	tlsConfig.InsecureSkipVerify = skipVerify
	caPem, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPem) {
		return nil, fmt.Errorf("failed to append certificates")
	}
	tlsConfig.RootCAs = certPool

	if clientKeyPath != "" && clientCertChainPath != "" {
		if err := checkTLSFile(clientKeyPath); err != nil {
			return nil, err
		}
		if err := checkTLSFile(clientCertChainPath); err != nil {
			return nil, err
		}
		clientPem, err := tls.LoadX509KeyPair(clientCertChainPath, clientKeyPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{clientPem}
	}
	return credentials.NewTLS(tlsConfig), nil
}

// checkTLSFile checks the TLS files.
func checkTLSFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() == 0 {
		return fmt.Errorf("the TLS file is illegal: %s", path)
	}
	return nil
}
