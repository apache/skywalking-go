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

FROM golang:{{.Context.GoVersion}}

WORKDIR /skywalking-go
COPY . .

{{ if .Context.DebugMode -}}
RUN mkdir -p /gotmp
{{ end -}}
{{ if .GreaterThanGo18 -}}
RUN go work use test/plugins/workspace/{{.Context.ScenarioName}}/{{.Context.CaseName}}
{{ else }}
RUN echo "replace github.com/apache/skywalking-go => ../../../../../" >> test/plugins/workspace/{{.Context.ScenarioName}}/{{.Context.CaseName}}/go.mod
{{ end -}}

WORKDIR /skywalking-go/test/plugins/workspace/{{.Context.ScenarioName}}/{{.Context.CaseName}}/
{{ if .Context.Config.Toolkit -}}
RUN echo "replace github.com/apache/skywalking-go/toolkit => ../../../../../toolkit" >> ./go.mod
{{ end }}
RUN go mod tidy

ENV GO_BUILD_OPTS=" -toolexec \"/skywalking-go{{.ToolExecPath}}\" -a -work "

CMD ["bash", "{{.Context.Config.StartScript}}"]
