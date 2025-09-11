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

exitOnError() {
    echo -e "\033[31m[ERROR] $1\033[0m">&2
    exit 1
}

dumpDocker() {
    phase=$1
    echo "[DOCKER-$phase] Enumerating containers..."
    if command -v docker >/dev/null 2>&1; then
        echo "[DOCKER-$phase] docker ps -a"
        docker ps -a || true
        echo "[DOCKER-$phase] Container states and health:"
        local ps_fmt='{{"{{"}}.Names{{"}}"}}'
        local inspect_fmt='{{"{{"}}.Name{{"}}"}} state={{"{{"}}.State.Status{{"}}"}} health={{"{{"}}if .State.Health{{"}}"}}{{"{{"}}.State.Health.Status{{"}}"}}{{"{{"}}else{{"}}"}}n/a{{"{{"}}end{{"}}"}}'
        for name in $(docker ps -a --format "$ps_fmt"); do
            docker inspect -f "$inspect_fmt" "$name" || true
        done
        echo "[DOCKER-$phase] Recent logs (last 200 lines) for each container:"
        for name in $(docker ps -a --format "$ps_fmt"); do
            echo "------ logs: $name (tail -200) ------"
            docker logs --tail 200 "$name" || true
        done
    else
        echo "[DOCKER-$phase] docker CLI not available in this container; skipping."
    fi
}

healthCheck() {
    HEALTH_CHECK_URL=$1
    echo "[DEBUG] Starting health check for URL: ${HEALTH_CHECK_URL}" >&2
    STATUS=""
    TIMES=${TIMES:-120}
    i=1
    while [[ $i -lt ${TIMES} ]];
    do
        STATUS=$(curl --max-time 5 -is ${HEALTH_CHECK_URL} | grep -oE "HTTP/.*\s+200")
        if [[ -n "$STATUS" ]]; then
          echo "[DEBUG] Success: ${HEALTH_CHECK_URL}: ${STATUS}" >&2
          return 0
        fi
        sleep 3
        i=$(($i + 1))
    done

    echo "[ERROR] Health check failed after $TIMES attempts for ${HEALTH_CHECK_URL}." >&2
    echo "[DEBUG] Resolver/hosts debug:" >&2
    cat /etc/resolv.conf || true >&2
    getent hosts || true >&2
    echo "[DEBUG] Verbose curl output:" >&2
    curl -v --max-time 5 -is ${HEALTH_CHECK_URL} || true >&2
    exitOnError "{{.Context.ScenarioName}}-{{.Context.CaseName}} url=${HEALTH_CHECK_URL}, status=${STATUS} health check failed!"
}
HTTP_HOST=127.0.0.1
HTTP_PORT={{.Context.Config.ExportPort}}

# Container uses network_mode: host, so 127.0.0.1 should work directly
echo "[DEBUG] Waiting 10s for Windows service to start..."
sleep 10

# Debug network connectivity from container side
echo "[DEBUG] Container network debug:"
echo "[DEBUG] - Container hostname: $(hostname)"
echo "[DEBUG] - Container IP addresses:"
ip addr show || ifconfig || echo "No ip/ifconfig available"
echo "[DEBUG] - /etc/hosts content:"
cat /etc/hosts
echo "[DEBUG] - /etc/resolv.conf content:"
cat /etc/resolv.conf
echo "[DEBUG] - Testing localhost resolution:"
nslookup localhost || echo "nslookup failed"
echo "[DEBUG] - Testing 127.0.0.1 connectivity:"
ping -c 1 127.0.0.1 || echo "ping 127.0.0.1 failed"
echo "[DEBUG] - Testing port 8080 on 127.0.0.1:"
nc -zv 127.0.0.1 8080 || telnet 127.0.0.1 8080 || echo "Port 8080 not reachable"

# Sync expected file host to match the actual HTTP_HOST
if [ -f /workspace/config/excepted.yml ]; then
  sed -i "s#service:8080#${HTTP_HOST}:8080#g" /workspace/config/excepted.yml || true
fi

echo "Checking the service health status..."
dumpDocker "before-healthcheck"
healthCheck "{{.Context.Config.HealthChecker}}"
dumpDocker "after-healthcheck"

echo "Visiting entry service..."
`echo curl -s --max-time 3 {{.Context.Config.EntryService}}` || true
sleep 5

echo "Receiving actual data..."
curl -s --max-time 3 http://localhost:12800/receiveData > /workspace/config/actual.yaml
[[ ! -f /workspace/config/actual.yaml ]] && exitOnError "{{.Context.ScenarioName}}-{{.Context.CaseName}}, 'actual.yaml' Not Found!"

echo "Validating actual data..."
response=$(curl -X POST --data-binary "@/workspace/config/excepted.yml" -s -w "\n%{http_code}" http://localhost:12800/dataValidate)
status_code=$(echo "$response" | tail -n1)
response_body=$(echo "$response" | head -n -1)
if [ "$status_code" -ne 200 ]; then
  exitOnError "{{.Context.ScenarioName}}-{{.Context.CaseName}}, validate actual data failed! \n$response_body"
fi

exit 0