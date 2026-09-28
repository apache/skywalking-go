#!/usr/bin/env bash
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Static multi-backend failover:
# 1) resolve active via unique GET:/sw-failover-probe/{token} span (consumer-only)
# 2) kill that compose service
# 3) assert consumer-specific standby growth (toolkit log + new unique probe),
#    not POST:/info alone (provider may already be on standby and false-pass)
#
# Keep waits short: long probe polls × infra-e2e retries previously hit the
# 90m GHA job timeout. Idempotent after the active collector was killed.

set -euo pipefail

CONSUMER_HOST="http://${consumer_host}:${consumer_8080}"
A_DATA="http://${collector_a_host}:${collector_a_12800}/receiveData"
B_DATA="http://${collector_b_host}:${collector_b_12800}/receiveData"

# Emitted by consumer /info via toolkit/logging (see test/e2e/base/consumer).
LOG_NEEDLE="this is info msg"

running_container_id() {
  local svc="$1"
  if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
    docker ps -q \
      -f "label=com.docker.compose.project=${COMPOSE_PROJECT_NAME}" \
      -f "label=com.docker.compose.service=${svc}"
  else
    docker ps -q -f "label=com.docker.compose.service=${svc}"
  fi
}

collector_data() {
  curl -sf --max-time 5 "$1" 2>/dev/null || true
}

has_needle() {
  local data_url="$1"
  local needle="$2"
  echo "$(collector_data "${data_url}")" | grep -q "${needle}"
}

count_needle() {
  local data_url="$1"
  local needle="$2"
  echo "$(collector_data "${data_url}")" | grep -c "${needle}" || true
}

send_info() {
  local times="${1:-5}"
  local i
  for ((i = 0; i < times; i++)); do
    curl -sf --max-time 5 -X POST "${CONSUMER_HOST}/info" >/dev/null || true
    sleep 2
  done
}

send_unique_probe() {
  PROBE_TOKEN="p$(date +%s%N | tail -c 10)${RANDOM}"
  PROBE_PATH="/sw-failover-probe/${PROBE_TOKEN}"
  PROBE_NEEDLE="GET:${PROBE_PATH}"
  curl -sf --max-time 5 "${CONSUMER_HOST}${PROBE_PATH}" >/dev/null || true
}

dump_debug() {
  local data_url="$1"
  echo "standby /receiveData dump (truncated):" >&2
  collector_data "${data_url}" | head -c 6000 >&2 || true
  echo >&2
  local cid
  cid="$(running_container_id consumer || true)"
  if [[ -n "${cid}" ]]; then
    echo "consumer logs (tail):" >&2
    docker logs --tail 80 "${cid}" >&2 || true
  fi
}

assert_standby() {
  local data_url="$1"
  local before_log after_log before_probe after_probe
  # Consumer-specific only: toolkit log growth + a fresh unique probe after kill.
  # Do not rely on POST:/info — provider may already report that on standby.
  before_log="$(count_needle "${data_url}" "${LOG_NEEDLE}")"
  send_unique_probe
  before_probe="$(count_needle "${data_url}" "${PROBE_NEEDLE}")"
  send_info 5
  # Re-send the same unique probe plus traffic so consumer Collect can fail over.
  for ((i = 0; i < 5; i++)); do
    curl -sf --max-time 5 "${CONSUMER_HOST}${PROBE_PATH}" >/dev/null || true
    sleep 2
  done
  sleep 10
  after_log="$(count_needle "${data_url}" "${LOG_NEEDLE}")"
  after_probe="$(count_needle "${data_url}" "${PROBE_NEEDLE}")"

  if [[ "${after_log}" -le "${before_log}" ]]; then
    echo "standby missing post-failover consumer log growth (${LOG_NEEDLE} ${before_log}->${after_log}): ${data_url}" >&2
    dump_debug "${data_url}"
    exit 1
  fi
  if [[ "${after_probe}" -le "${before_probe}" ]]; then
    echo "standby missing post-failover consumer probe growth (${PROBE_NEEDLE} ${before_probe}->${after_probe}): ${data_url}" >&2
    dump_debug "${data_url}"
    exit 1
  fi

  echo "standby ok consumer log ${before_log}->${after_log} probe ${before_probe}->${after_probe}" >&2
  echo "status: ok"
}

running_a="$(running_container_id collector_a | wc -l | tr -d ' ')"
running_b="$(running_container_id collector_b | wc -l | tr -d ' ')"

if [[ "${running_a}" -eq 1 && "${running_b}" -eq 1 ]]; then
  send_unique_probe
  a_has=0
  b_has=0
  # Bound active resolution (~24s) so pending retries stay within job timeout.
  local_i=0
  for ((local_i = 0; local_i < 12; local_i++)); do
    a_has=0
    b_has=0
    has_needle "${A_DATA}" "${PROBE_NEEDLE}" && a_has=1
    has_needle "${B_DATA}" "${PROBE_NEEDLE}" && b_has=1
    if [[ "${a_has}" -eq 1 || "${b_has}" -eq 1 ]]; then
      break
    fi
    sleep 2
  done

  if [[ "${a_has}" -eq 0 && "${b_has}" -eq 0 ]]; then
    echo "pending: neither collector has ${PROBE_NEEDLE} yet" >&2
    exit 1
  fi
  if [[ "${a_has}" -eq 1 && "${b_has}" -eq 1 ]]; then
    echo "ambiguous: both collectors received ${PROBE_NEEDLE}" >&2
    exit 1
  fi

  if [[ "${a_has}" -eq 1 ]]; then
    active_svc=collector_a
    standby_url="${B_DATA}"
  else
    active_svc=collector_b
    standby_url="${A_DATA}"
  fi

  echo "killing active backend ${active_svc} (probe ${PROBE_NEEDLE})" >&2
  docker kill "$(running_container_id "${active_svc}")" >/dev/null
  # Allow the transport and native pick_first to reconnect to the standby.
  sleep 20
  assert_standby "${standby_url}"
elif [[ "${running_a}" -eq 1 ]]; then
  assert_standby "${A_DATA}"
elif [[ "${running_b}" -eq 1 ]]; then
  assert_standby "${B_DATA}"
else
  echo "no collector containers running" >&2
  exit 1
fi
