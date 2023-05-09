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
	// imports required packages for gRPC reporter
	_ "context"
	_ "fmt"
	_ "io"
	_ "os"
	_ "strconv"
	_ "time"

	// imports the logs for reporter
	_ "github.com/apache/skywalking-go/agent/core/operator"
	_ "github.com/apache/skywalking-go/log"

	// imports configuration and starter for gRPC
	_ "google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer"
	_ "google.golang.org/grpc/balancer/grpclb"
	_ "google.golang.org/grpc/balancer/roundrobin"
	_ "google.golang.org/grpc/balancer/weightedroundrobin"
	_ "google.golang.org/grpc/balancer/weightedtarget"
	_ "google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/connectivity"
	_ "google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/gzip"
	_ "google.golang.org/grpc/grpclog"
	_ "google.golang.org/grpc/keepalive"
	_ "google.golang.org/grpc/metadata"
	_ "google.golang.org/grpc/resolver"
	_ "google.golang.org/grpc/resolver/manual"
	_ "google.golang.org/grpc/stats"
	_ "google.golang.org/grpc/status"

	// imports protocols between agent and backend
	_ "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	_ "skywalking.apache.org/repo/goapi/collect/common/v3"
	_ "skywalking.apache.org/repo/goapi/collect/ebpf/profiling/process/v3"
	_ "skywalking.apache.org/repo/goapi/collect/ebpf/profiling/v3"
	_ "skywalking.apache.org/repo/goapi/collect/event/v3"
	_ "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	_ "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
	_ "skywalking.apache.org/repo/goapi/collect/logging/v3"
	_ "skywalking.apache.org/repo/goapi/collect/management/v3"
	_ "skywalking.apache.org/repo/goapi/collect/servicemesh/v3"
)
