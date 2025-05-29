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

FROM golang:1.19 as builder

ENV CGO_ENABLED=0
ENV GO111MODULE=on

WORKDIR /build

COPY . /build/skywalking-go
COPY test/e2e/base/provider /build/provider
COPY test/e2e/dist/skywalking-go-agent /build/skywalking-go-agent
RUN chmod +x /build/skywalking-go-agent

WORKDIR /build/provider
RUN go mod edit -replace=github.com/apache/skywalking-go=../skywalking-go
RUN go mod edit -replace=github.com/apache/skywalking-go/toolkit=../skywalking-go/toolkit

RUN go mod tidy && go build -toolexec="/build/skywalking-go-agent" -a -o provider

FROM alpine:3.11

WORKDIR /app

COPY --from=builder /build/provider/provider /provider

CMD ["/provider"]
