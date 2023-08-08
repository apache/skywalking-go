#!/bin/bash
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

home="$(cd "$(dirname $0)";cd ..; pwd)"
cd $home

echo "Installing protoc"
apt update && apt install -y protobuf-compiler
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
export PATH="$PATH:$(go env GOPATH)/bin"

echo "building API"
protoc -I=${home}/api --go_out=${home}/api --go-grpc_out=${home}/api ${home}/api/api.proto

echo "building applications"
go build ${GO_BUILD_OPTS} -o server ./grpc_server/server.go
go build ${GO_BUILD_OPTS} -o client ./grpc_client/client.go


echo "starting server"
export SW_AGENT_NAME=grpc-server
./server &
sleep 2

echo "starting client"
export SW_AGENT_NAME=grpc-client
./client
