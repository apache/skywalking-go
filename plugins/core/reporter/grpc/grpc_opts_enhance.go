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

package grpc

import (
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

var authKey = "Authentication"

// ReporterOption allows for functional options to adjust behavior
// of a gRPC reporter to be created by NewGRPCReporter
type ReporterOption func(r *gRPCReporter)

// WithCheckInterval setup service and endpoint registry check interval
func WithCheckInterval(interval time.Duration) ReporterOption {
	return func(r *gRPCReporter) {
		r.checkInterval = interval
	}
}

// WithMaxSendQueueSize setup send span queue buffer length
func WithMaxSendQueueSize(maxSendQueueSize int) ReporterOption {
	return func(r *gRPCReporter) {
		r.tracingSendCh = make(chan *agentv3.SegmentObject, maxSendQueueSize)
	}
}

// WithTransportCredentials setup transport layer security
func WithTransportCredentials(creds credentials.TransportCredentials) ReporterOption {
	return func(r *gRPCReporter) {
		r.creds = creds
	}
}

// WithAuthentication used Authentication for gRPC
func WithAuthentication(auth string) ReporterOption {
	return func(r *gRPCReporter) {
		r.md = metadata.New(map[string]string{authKey: auth})
	}
}

// WithCDS setup Configuration Discovery Service to dynamic config
func WithCDS(interval time.Duration) ReporterOption {
	return func(r *gRPCReporter) {
		r.cdsInterval = interval
	}
}
