#
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
#

REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))/../../

OUT_DIR = $(REPODIR)/bin
BINARY = skywalking-go-agent

SH = sh
GO = go
GIT = git
PROTOC = protoc
GO_PATH = $$($(GO) env GOPATH)
GO_BUILD = $(GO) build
GO_LINT = $(GO_PATH)/bin/golangci-lint
GO_BUILD_FLAGS = -v
GO_BUILD_LDFLAGS = -X main.version=$(VERSION)
GO_TEST_LDFLAGS =
GO_GET = $(GO) get

PLATFORMS := linux darwin windows
os = $(word 1, $@)
ARCH ?= amd64

SHELL = /bin/bash

.PHONY: clean
clean: tools
	-rm -rf coverage.txt

deps: tools
	$(GO_GET) -v -t -d ./...

.PHONY: build
build: linux darwin windows

.PHONY: $(PLATFORMS)
$(PLATFORMS):
	mkdir -p $(OUT_DIR)
	GOOS=$(os) GOARCH=$(ARCH) $(GO_BUILD) $(GO_BUILD_FLAGS) -ldflags "$(GO_BUILD_LDFLAGS)" -o $(OUT_DIR)/$(BINARY)-$(VERSION)-$(os)-$(ARCH) ./cmd
