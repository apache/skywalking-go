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
go mod tidy
build_shell="go build ${GO_BUILD_OPTS} -o ${project_name} main.go"
echo "Building the project..."
eval $build_shell
export SW_AGENT_NAME=${project_name}
export SW_AGENT_REPORTER_GRPC_BACKEND_SERVICE=localhost:19876
export SW_AGENT_METER_COLLECT_INTERVAL=1
export SW_AGENT_REPORTER_CHECK_INTERVAL=5
eval "$(grep '^export ' ./bin/startup.sh)"

# HTTP server will listen on 127.0.0.1:8080 (default)
# Container uses network_mode: host, so it should access 127.0.0.1:8080 directly
echo "[DEBUG] Starting HTTP service: ./${project_name}"
./${project_name} &
web_pid=$!
echo "[DEBUG] HTTP service started with PID: $web_pid"

# Wait a moment and check if the service is running
sleep 2
if ps -p $web_pid > /dev/null; then
  echo "[DEBUG] HTTP service is running (PID: $web_pid)"
else
  echo "[ERROR] HTTP service failed to start or crashed immediately"
fi

# Check if port 8080 is listening
echo "[DEBUG] Checking if port 8080 is listening..."
netstat -an | grep :8080 || echo "[WARN] Port 8080 not found in netstat"

# Try to access the health endpoint from Windows side
echo "[DEBUG] Testing health endpoint from Windows side..."
curl -s http://127.0.0.1:8080/health && echo " - Health check OK" || echo " - Health check FAILED"

echo "[HOST] docker ps -a (before WSL run)"
docker ps -a || true
echo "[HOST] Logs tail (before) for ${project_name}-oap-1 and ${project_name}-validator-1"
for name in "${project_name}-oap-1" "${project_name}-validator-1"; do
  echo "------ logs: $name (tail -200) ------"
  docker logs --tail 200 "$name" || true
done

wsl-run.bat "${home}/wsl-scenarios.sh"

kill -9 $web_pid