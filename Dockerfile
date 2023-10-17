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

# User used targeting base image
ARG BASE_GO_IMAGE
# Build the agent base image
ARG BASE_BUILDER_IMAGE='golang:1.18'

FROM ${BASE_BUILDER_IMAGE} as builder
# Go Agent Version
ARG VERSION
# Current ARCH
ARG TARGETARCH

WORKDIR /skywalking-go
COPY . .
RUN VERSION=$VERSION ARCH=$TARGETARCH make -C tools/go-agent linux

FROM ${BASE_GO_IMAGE}

COPY --from=builder /skywalking-go/bin/skywalking-go-agent*linux* /usr/local/bin/skywalking-go-agent
