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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	configuration "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	managementv3 "skywalking.apache.org/repo/goapi/collect/management/v3"
)

//skywalking:nocopy

// All struct are from the reporter package in the library, copy these files is works for compiler

// nolint
type Entity struct {
	ServiceName         string
	ServiceInstanceName string
	Props               []*commonv3.KeyStringValuePair
	Layer               string
}

// nolint
type gRPCReporter struct {
	entity           Entity
	sendCh           chan *agentv3.SegmentObject
	conn             *grpc.ClientConn
	traceClient      agentv3.TraceSegmentReportServiceClient
	managementClient managementv3.ManagementServiceClient
	checkInterval    time.Duration
	cdsInterval      time.Duration
	cdsClient        configuration.ConfigurationDiscoveryServiceClient

	md    metadata.MD
	creds credentials.TransportCredentials
}

// GRPCReporterOption allows for functional options to adjust behavior
// of a gRPC reporter to be created by NewGRPCReporter
type GRPCReporterOption func(r *gRPCReporter)
