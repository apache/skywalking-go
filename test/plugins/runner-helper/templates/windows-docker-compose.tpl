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

version: '2.1'

services:
  oap:
    image: ghcr.io/apache/skywalking-agent-test-tool/mock-collector:fa81b1b6d9caef484a65b5019efa28cac4e3d21d
    network_mode: host
    expose:
      - 19876
      - 12800
      - 11800
    healthcheck:
      test: ["CMD", "bash", "-c", "cat < /dev/null > /dev/tcp/127.0.0.1/12800"]
      interval: 5s
      timeout: 60s
      retries: 120
  validator:
    image: skywalking/agent-test-validator:1.0.0
    network_mode: host
    depends_on:
      oap:
        condition: service_healthy
    volumes:
      - {{.Context.WorkSpaceDir}}:/workspace
    command: ["/bin/bash", "-c", "sleep 10s && /workspace/validator.sh"]
  {{- range $name, $service := .Context.Config.Dependencies }}
  {{$name}}:
    image: {{$service.Image}}
    {{- if $service.Hostname }}
    hostname: {{$service.Hostname}}
    {{- end }}
    {{- if $service.Ports }}
    ports:
      {{- range $service.Ports }}
      - "{{.}}"
      {{- end }}
    {{- end }}
    {{- if $service.Exports }}
    expose:
      {{- range $service.Exports }}
      - "{{.}}"
      {{- end }}
    {{- end }}
    {{- if $service.Env }}
    environment:
      {{- range $key, $value := $service.Env }}
      {{$key}}: {{$value}}
      {{- end }}
    {{- end }}
    {{- if $service.Command }}
    command:
      {{- range $service.Command }}
      - "{{.}}"
      {{- end }}
    {{- end }}
    {{- if $service.Volumes }}
    volumes:
      {{- range $service.Volumes }}
      - "{{.}}"
      {{- end }}
    {{- end }}
    {{- if $service.DependsOn }}
    depends_on:
      {{- range $service.DependsOn }}
      - "{{.}}"
      {{- end }}
    {{- end }}
    {{- if $service.HealthCheck }}
    healthcheck:
      test:
        {{- range $service.HealthCheck.Test }}
        - "{{.}}"
        {{- end }}
      interval: {{$service.HealthCheck.Interval}}
      timeout: {{$service.HealthCheck.Timeout}}
      retries: {{$service.HealthCheck.Retries}}
    {{- end }}
  {{- end }}