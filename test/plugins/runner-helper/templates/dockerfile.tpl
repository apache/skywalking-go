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
# `go mod tidy` ignores go.work, so it would otherwise resolve
# github.com/apache/skywalking-go to the latest released version from the module
# proxy instead of the code under test. Pin it to the local module explicitly.
# `go mod edit` is used instead of appending the directive, because not every
# scenario go.mod ends with a newline.
RUN go mod edit -replace=github.com/apache/skywalking-go=../../../../../ test/plugins/workspace/{{.Context.ScenarioName}}/{{.Context.CaseName}}/go.mod
# google.golang.org/genproto moved googleapis/rpc/* into the separate
# google.golang.org/genproto/googleapis/rpc module on 2023-05-30. Older
# scenarios drag the pre-split monolith into the graph, where it collides with
# the split module that google.golang.org/grpc requires, and every import of
# googleapis/rpc/* then fails with "ambiguous import". Raise the monolith to the
# first version that no longer carries those packages so the two can coexist.
# Scenarios that do not need the monolith at all drop it again during tidy.
RUN go mod edit -require=google.golang.org/genproto@v0.0.0-20230530153820-e85fd2cbaebc test/plugins/workspace/{{.Context.ScenarioName}}/{{.Context.CaseName}}/go.mod
{{ if .GreaterThanGo18 -}}
RUN go work use test/plugins/workspace/{{.Context.ScenarioName}}/{{.Context.CaseName}}
{{ end -}}

WORKDIR /skywalking-go/test/plugins/workspace/{{.Context.ScenarioName}}/{{.Context.CaseName}}/
{{ if .Context.Config.Toolkit -}}
RUN go mod edit -replace=github.com/apache/skywalking-go/toolkit=../../../../../toolkit
{{ end }}
RUN go mod tidy
{{ if .GreaterThanGo18 -}}
# go mod tidy may raise the scenario module's Go directive to a patch release
# (for example 1.25.0). Refresh the workspace directive so Go does not reject
# the generated module as requiring a newer version than go.work.
RUN go work use .
{{ end }}

ENV GO_BUILD_OPTS=" -toolexec \"/skywalking-go{{.ToolExecPath}}\" -a -work "

CMD ["bash", "{{.Context.Config.StartScript}}"]
