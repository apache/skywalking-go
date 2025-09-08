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

healthCheck() {
    HEALTH_CHECK_URL=$1
    STATUS=""
    TIMES=${TIMES:-60}
    i=1
    while [[ $i -lt ${TIMES} ]];
    do
        echo "[healthcheck] attempt=${i}/${TIMES} url=${HEALTH_CHECK_URL}"
        host_part=${HEALTH_CHECK_URL#*://}
        host_part=${host_part%%[:/]*}
        echo "[healthcheck] resolving host='${host_part}'"
        getent hosts "${host_part}" || true
        echo "[healthcheck] curl timings (code connect starttransfer total) ->" \
             $(curl -s -o /dev/null -w "%{http_code} %{time_connect} %{time_starttransfer} %{time_total}" "${HEALTH_CHECK_URL}" || echo "curl_failed")
        STATUS=$(curl --max-time 3 -is ${HEALTH_CHECK_URL} | grep -oE "HTTP/.*\s+200") || true
        if [[ -n "$STATUS" ]]; then
          echo "${HEALTH_CHECK_URL}: ${STATUS}"
          return 0
        fi
        sleep 3
        i=$(($i + 1))
    done

    exitOnError "{{.Context.ScenarioName}}-{{.Context.CaseName}} url=${HEALTH_CHECK_URL}, status=${STATUS} health check failed!"
}

HTTP_HOST=127.0.0.1
HTTP_PORT={{.Context.Config.ExportPort}}

echo "Checking the service health status..."
healthCheck "{{.Context.Config.HealthChecker}}"

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