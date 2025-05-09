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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/connectivity"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var authKey = "Authentication"

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
}

type ManagedConnection struct {
	connection *grpc.ClientConn
	status     ConnectionStatus
	refCount   int
}

func (cm *ConnectionManager) GetMD() metadata.MD {
	return cm.md
}

func (cm *ConnectionManager) GetConnection(serverAddr string) (*grpc.ClientConn, error) {
	managed, exists := cm.connManager[serverAddr]
	if exists {
		managed.refCount++
		return managed.connection, nil
	}
	conn, err := cm.createConnection()
	if err != nil {
		return nil, err
	}
	managed = &ManagedConnection{
		connection: conn,
		status:     ConnectionStatusConnected,
		refCount:   1,
	}
	cm.connManager[serverAddr] = managed
	go cm.checkConnectionStatus(serverAddr)
	return conn, nil
}

func (cm *ConnectionManager) createConnection() (*grpc.ClientConn, error) {
	var credsDialOption grpc.DialOption
	if cm.creds != nil {
		// use tls
		credsDialOption = grpc.WithTransportCredentials(cm.creds)
	} else {
		credsDialOption = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.Dial(cm.serverAddr, credsDialOption, grpc.WithConnectParams(grpc.ConnectParams{
		// update the max backoff delay interval
		Backoff: backoff.Config{
			BaseDelay:  1.0 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   cm.checkInterval,
		},
	}))
	return conn, err
}

func (cm *ConnectionManager) checkConnectionStatus(serverAddr string) {
	for {
		cm.mu.Lock()
		managed, exists := cm.connManager[serverAddr]
		cm.mu.Unlock()
		if !exists {
			return
		}
		state := managed.connection.GetState()
		var newStatus ConnectionStatus
		switch state {
		case connectivity.TransientFailure:
			newStatus = ConnectionStatusDisconnect
		case connectivity.Shutdown:
			newStatus = ConnectionStatusShutdown
		default:
			newStatus = ConnectionStatusConnected
		}
		if newStatus != managed.status {
			cm.mu.Lock()
			managed.status = newStatus
			cm.mu.Unlock()
		}
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

func (cm *ConnectionManager) GetConnectionStatus(serverAddr string) ConnectionStatus {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	managed, exists := cm.connManager[serverAddr]
	if !exists {
		return ConnectionStatusShutdown
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
