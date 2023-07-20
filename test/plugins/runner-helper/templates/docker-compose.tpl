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

networks:
  default:
    name: {{.Context.ScenarioName}}

services:
  oap:
    image: ghcr.io/apache/skywalking-agent-test-tool/mock-collector:fa81b1b6d9caef484a65b5019efa28cac4e3d21d
    expose:
      - 19876
      - 12800
    ports:
      - 12800
    healthcheck:
      test: ["CMD", "bash", "-c", "cat < /dev/null > /dev/tcp/127.0.0.1/12800"]
      interval: 5s
      timeout: 60s
      retries: 120
  service:
    build:
      context: {{.Context.ProjectDir}}
      dockerfile: {{.DockerFilePathRelateToProject}}
    depends_on:
      oap:
        condition: service_healthy
      {{- if .Context.Config.Dependencies }}
      {{- range $name, $service := .Context.Config.Dependencies }}
      {{$name}}:
        condition: {{ if $service.HealthCheck -}} service_healthy {{- else -}} service_started {{- end}}
      {{- end }}
      {{- end }}
    ports:
      - {{.Context.Config.ExportPort}}
    {{ if .Context.DebugMode -}}
    volumes:
      - {{.Context.WorkSpaceDir}}/gobuild:/gotmp
    {{ end -}}
    environment:
      SW_AGENT_NAME: {{.Context.ScenarioName}}
      SW_AGENT_REPORTER_GRPC_BACKEND_SERVICE: oap:19876
      {{ if .Context.DebugMode -}}
      GOTMPDIR: /gotmp
      {{- end }}
    healthcheck:
      test: ["CMD", "bash", "-c", "cat < /dev/null > /dev/tcp/127.0.0.1/{{.Context.Config.ExportPort}}"]
      interval: 5s
      timeout: 60s
      retries: 120
  validator:
    image: skywalking/agent-test-validator:1.0.0
    depends_on:
      service:
        condition: service_healthy
    volumes:
      - {{.Context.WorkSpaceDir}}:/workspace
    command: ["/bin/bash", "/workspace/validator.sh"]
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