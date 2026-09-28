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
	"google.golang.org/grpc/keepalive"
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

// multiBackendPerAddrDialTimeout caps each TCP dial so a blackholed first
// address cannot consume the shared pick_first MinConnectTimeout alone.
const multiBackendPerAddrDialTimeout = 2 * time.Second

var agentKeepaliveParams = keepalive.ClientParameters{
	Time:                30 * time.Second,
	Timeout:             10 * time.Second,
	PermitWithoutStream: true,
}

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

		conn, err := grpc.Dial(cm.serverAddr, credsDialOption,
			grpc.WithKeepaliveParams(agentKeepaliveParams),
			grpc.WithConnectParams(grpc.ConnectParams{
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
	opts = append(opts,
		grpc.WithKeepaliveParams(agentKeepaliveParams),
		grpc.WithContextDialer(multiBackendContextDialer(multiBackendPerAddrDialTimeout)),
		grpc.WithConnectParams(grpc.ConnectParams{
			MinConnectTimeout: multiBackendDialTimeout,
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   cm.checkInterval,
			},
		}),
	)
	conn, dialErr := grpc.Dial(target, opts...)
	if dialErr != nil {
		if cm.logger != nil {
			cm.logger.Errorf("dial multi-backend target %q failed: %v", target, dialErr)
		}
		return nil, fmt.Errorf("dial backend %q via static multi-backend resolver: %w", target, dialErr)
	}
	return conn, nil
}

// multiBackendContextDialer gives each pick_first address its own dial/handshake
// budget so one silent peer cannot starve the rest of MinConnectTimeout.
// TCP connect is capped by DialContext; Accept-never-called peers still pass
// Dial but hang on the HTTP/2 preface read — a temporary conn deadline bounds
// that hang and is cleared once the peer responds.
func multiBackendContextDialer(perAddrTimeout time.Duration) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		dialCtx, cancel := context.WithTimeout(ctx, perAddrTimeout)
		defer cancel()
		raw, err := d.DialContext(dialCtx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		deadline := time.Now().Add(perAddrTimeout)
		if err := raw.SetDeadline(deadline); err != nil {
			_ = raw.Close()
			return nil, err
		}
		return &handshakeBoundedConn{Conn: raw}, nil
	}
}

// handshakeBoundedConn clears the dialer-imposed deadline after the peer
// responds (first successful Read), so long-lived Collect streams are unbound.
type handshakeBoundedConn struct {
	net.Conn
	cleared atomic.Bool
}

func (c *handshakeBoundedConn) clearDeadline() {
	if c.cleared.CompareAndSwap(false, true) {
		_ = c.Conn.SetDeadline(time.Time{})
	}
}

func (c *handshakeBoundedConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.clearDeadline()
	}
	return n, err
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

// boundSendTimeoutNs is atomic so tests can shorten BoundSend without racing
// pipeline goroutines that still read the default.
var boundSendTimeoutNs atomic.Int64

func init() {
	boundSendTimeoutNs.Store(int64(60 * time.Second))
}

func boundSendTimeout() time.Duration {
	return time.Duration(boundSendTimeoutNs.Load())
}

// BoundSendTimeoutForTest / SetBoundSendTimeoutForTest let unit tests exercise
// kill→standby without waiting the production BoundSend timeout.
func BoundSendTimeoutForTest() time.Duration { return boundSendTimeout() }
func SetBoundSendTimeoutForTest(d time.Duration) {
	boundSendTimeoutNs.Store(int64(d))
}

// BoundSendWatchdog reuses a single timer across many Send/CloseAndRecv calls
// on one Collect stream (Reset instead of allocating AfterFunc per message).
type BoundSendWatchdog struct {
	cancel  context.CancelFunc
	timeout time.Duration
	timer   *time.Timer
}

// NewBoundSendWatchdog prepares a stopped watchdog. Pass timeout<=0 to use the
// configured BoundSend default. Stop must be called when the stream ends.
func NewBoundSendWatchdog(cancel context.CancelFunc, timeout time.Duration) *BoundSendWatchdog {
	if timeout <= 0 {
		timeout = boundSendTimeout()
	}
	w := &BoundSendWatchdog{cancel: cancel, timeout: timeout}
	if cancel != nil {
		w.timer = time.AfterFunc(time.Hour, cancel)
		w.timer.Stop()
	}
	return w
}

// Do runs send under the watchdog; cancel fires if send exceeds timeout.
// Panics from send propagate to the caller for sendWithRecover.
func (w *BoundSendWatchdog) Do(send func() error) error {
	if w == nil || w.cancel == nil || w.timer == nil {
		return send()
	}
	_ = w.timer.Stop()
	w.timer.Reset(w.timeout)
	defer w.timer.Stop()
	return send()
}

// Stop releases the timer. Safe to call more than once.
func (w *BoundSendWatchdog) Stop() {
	if w == nil || w.timer == nil {
		return
	}
	w.timer.Stop()
}

// BoundSend is a one-shot watchdog for tests and CloseAndRecv helpers.
// Prefer BoundSendWatchdog.Do on Collect hot paths.
func BoundSend(cancel context.CancelFunc, send func() error, timeout time.Duration) error {
	w := NewBoundSendWatchdog(cancel, timeout)
	defer w.Stop()
	return w.Do(send)
}

// BackendRPCContext returns the context for an outbound unary backend RPC.
func BackendRPCContext(atLeast time.Duration) (context.Context, context.CancelFunc) {
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
func BackendStreamContext(atLeast time.Duration) (
	ctx context.Context, cancel context.CancelFunc, stopOpenTimer func() (timedOut bool),
) {
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
// leaves Ready (Idle, TransientFailure, or Shutdown). pick_first often moves
// to Idle before TransientFailure when a peer goes dark; keepalive accelerates
// detection, and canceling Idle after seenReady unblocks hung Send.
// Start only after Collect succeeds so Connecting → Ready on a standby is not
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
		if seenReady && (state == connectivity.Idle ||
			state == connectivity.TransientFailure ||
			state == connectivity.Shutdown) {
			cancel()
			return
		}
		if !conn.WaitForStateChange(ctx, state) {
			return
		}
	}
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
		cm.mu.Unlock()

		state := conn.GetState()
		var newStatus ConnectionStatus
		switch state {
		case connectivity.TransientFailure:
			newStatus = ConnectionStatusDisconnect
		case connectivity.Shutdown:
			newStatus = ConnectionStatusShutdown
		default:
			// Idle, Connecting, and Ready all report Connected so pipeline loops
			// keep trying Collect; the next RPC wakes Idle without a Connect nudge.
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
		time.Sleep(5 * time.Second)
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
	// be between polls before it writes Shutdown into managed.status.
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
