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

home="$(cd "$(dirname $0)"; pwd)"
go install github.com/zeromicro/go-zero/tools/goctl@latest
goctl rpc protoc ./pb/user.proto --go_out=./pb/ --go-grpc_out=./pb/ --zrpc_out=. --style=go_zero

rm -rf etc internal/ user.go
sed -i 's|github.com/apache/skywalking-go/test/plugins/scenarios/gozero/pb/userpb|test/plugins/scenarios/gozero/pb/userpb|g' user/user.go

go build ${GO_BUILD_OPTS} -o gozero

export SW_AGENT_PLUGIN_CONFIG_GOZERO_COLLECT_REQUEST_PARAMETERS=true
export SW_AGENT_PLUGIN_CONFIG_GOZERO_COLLECT_LOGX=true


./gozero