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

set -ex

project_name=$(echo "{{.Context.ScenarioName}}" |sed -e "s/\.//g" |awk '{print tolower($0)}')

echo "Detected Windows OS"
home="$(cd "$(dirname $0)"; pwd)"
build_dir="$(cd "$(dirname $0)/../../.."; pwd)"
export GO_BUILD_OPTS="-toolexec=\"${build_dir}/dist/skywalking-go-agent.exe\" -a"
export GODEBUG="netdns=1"
go mod tidy
build_shell="go build ${GO_BUILD_OPTS} -o ${project_name} main.go"
echo "Building the project..."
eval $build_shell
export SW_AGENT_NAME=${project_name}
export SW_AGENT_REPORTER_GRPC_BACKEND_SERVICE=127.0.0.1:19876
eval "$(grep '^export ' ./bin/startup.sh)"

echo "Starting OAP server in WSL..."
wsl-run.bat "${home}/wsl-scenarios.sh" &
wsl_pid=$!


echo "Waiting for OAP server to be ready..."
for i in {1..60}; do
    if command -v nc >/dev/null 2>&1 && nc -z 127.0.0.1 19876 2>/dev/null; then
        echo "OAP server is ready!"
        break
    elif timeout 1 bash -c "</dev/tcp/127.0.0.1/19876" 2>/dev/null; then
        echo "OAP server is ready!"
        break
    fi
    sleep 2
    if [ $i -eq 60 ]; then
        echo "Timeout waiting for OAP server"
        exit 1
    fi
done

sleep 10

echo "Starting Windows application..."
./${project_name} &
web_pid=$!

wait $wsl_pid
wsl_exit_code=$?

kill -9 $web_pid

exit $wsl_exit_code