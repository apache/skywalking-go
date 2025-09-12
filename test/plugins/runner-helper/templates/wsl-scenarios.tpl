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
cp -rf /mnt/d/a/skywalking-go/skywalking-go/test/plugins/workspace /root/repo/skywalking-go/test/plugins/
cd {{.Context.WorkSpaceDir}}
echo "[DEBUG] WSL network debug:"
echo "[DEBUG] - /etc/resolv.conf content:"
cat /etc/resolv.conf || true
echo "[DEBUG] - WSL version:"
cat /proc/version || true
echo "[DEBUG] - Network interfaces (brief):"
echo "[DEBUG] --- ip route BEGIN ---"
if command -v ip >/dev/null 2>&1; then
  ip route | sed -n '1,120p'
  ip_rc=$?
  echo "[DEBUG] ip route exit=$ip_rc"
else
  echo "[WARN] 'ip' not found"
fi
echo "[DEBUG] --- ip route END ---"
echo "[DEBUG] --- ip addr BEGIN ---"
if command -v ip >/dev/null 2>&1; then
  ip -brief addr 2>&1 | sed -n '1,120p'
elif command -v ifconfig >/dev/null 2>&1; then
  ifconfig -a 2>&1 | sed -n '1,120p'
else
  echo "[WARN] neither 'ip' nor 'ifconfig' available"
fi
echo "[DEBUG] --- ip addr END ---"
echo "[DEBUG] - Route table:"
ip route 2>/dev/null || route -n 2>/dev/null || true

# Derive Windows host IP robustly: prefer WSL default gateway (Windows vEthernet address),
# then fallback to route table; DO NOT use resolv.conf nameserver
if command -v ip >/dev/null 2>&1; then
  WINDOWS_HOST=$(ip route | awk '/^default/ {print $3; exit}')
fi
if [ -z "$WINDOWS_HOST" ] && command -v route >/dev/null 2>&1; then
  WINDOWS_HOST=$(route -n | awk '/^0.0.0.0/ {print $2; exit}')
fi
if [ -z "$WINDOWS_HOST" ]; then
  WINDOWS_HOST="127.0.0.1"
  echo "[DEBUG] Fallback WINDOWS_HOST to 127.0.0.1"
fi
echo "[DEBUG] Resolved Windows host candidate IP: $WINDOWS_HOST"
export WINDOWS_HOST

# Keep validator.sh using Windows host IP for healthcheck
sed -i "s/HTTP_HOST=127\.0\.0\.1/HTTP_HOST=$WINDOWS_HOST/g" validator.sh


compose_version=$(docker-compose version --short)

if [[ $compose_version =~ ^(v)?1 ]]; then
    separator="_"
elif [[ $compose_version =~ ^(v)?2 ]]; then
    separator="-"
else
    echo "Unsupported Docker Compose version: $compose_version"
    exit 1
fi

project_name=$(echo "{{.Context.ScenarioName}}" |sed -e "s/\.//g" |awk '{print tolower($0)}')
validator_container_name="${project_name}${separator}validator${separator}1"

docker-compose -p "${project_name}" -f "docker-compose.yml" up -d --build
[[ $? -ne 0 ]] && exit 1
set -ex

sleep 5

validator_container_name="${project_name}${separator}validator${separator}1"

validator_container_id=`docker ps -aqf "name=${validator_container_name}"`

status=$(docker wait ${validator_container_id})

if [[ -z ${validator_container_id} ]]; then
    echo "docker startup failure!" >&2
    status=1
    echo "[WSL] docker ps -a (startup failure)" >&2
    docker ps -a || true >&2
    echo "[WSL] docker inspect (startup failure) for ${project_name}${separator}oap${separator}1 and ${validator_container_name}" >&2
    docker inspect "${project_name}${separator}oap${separator}1" "${validator_container_name}" || true >&2
    echo "[WSL] Logs tail (startup failure) for ${project_name}${separator}oap${separator}1 and ${validator_container_name}" >&2
    for name in "${project_name}${separator}oap${separator}1" "${validator_container_name}"; do
        echo "------ logs: $name (tail -200) ------" >&2
        docker logs --tail 200 "$name" || true >&2
    done
else
    [[ $status -ne 0 ]] && docker logs ${validator_container_id} >&2
    echo "[WSL] docker ps -a (before teardown)" >&2
    docker ps -a || true >&2
    echo "[WSL] docker inspect (before teardown) for ${project_name}${separator}oap${separator}1 and ${validator_container_name}" >&2
    docker inspect "${project_name}${separator}oap${separator}1" "${validator_container_name}" || true >&2
    echo "[WSL] Logs tail (before teardown) for ${project_name}${separator}oap${separator}1 and ${validator_container_name}" >&2
    for name in "${project_name}${separator}oap${separator}1" "${validator_container_name}"; do
        echo "------ logs: $name (tail -200) ------" >&2
        docker logs --tail 200 "$name" || true >&2
    done

    docker-compose -p ${project_name} -f "docker-compose.yml" kill
    docker-compose -p ${project_name} -f "docker-compose.yml" rm -f
fi

exit $status