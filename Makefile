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

SH = sh
GO = go
GIT = git
GO_PATH = $$($(GO) env GOPATH)
GO_BUILD = $(GO) build
GO_GET = $(GO) get
GO_LINT = $(GO_PATH)/bin/golangci-lint

GO_TEST = $(GO) test
GO_TEST_LDFLAGS =

REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))/

SHELL = /bin/bash

deps:
	$(GO_GET) -v -t -d ./...

linter:
	$(GO_LINT) version || curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GO_PATH)/bin v1.50.0

.PHONY: test
test:
	echo "mode: atomic" > ${REPODIR}/coverage.txt;
	@for dir in $$(find . -name go.mod -exec dirname {} \; ); do \
		cd $$dir; \
		go test -v -coverprofile=module_coverage.txt -covermode=atomic ./...; \
		test_status=$$?; \
		if [ -f module_coverage.txt ]; then \
			tail -n +2 module_coverage.txt >> ${REPODIR}/coverage.txt; \
			rm module_coverage.txt; \
		fi; \
		cd ${REPODIR}; \
		if [ $$test_status -ne 0 ]; then \
			echo "Error occurred during go test, exiting..."; \
			exit $$test_status; \
		fi; \
	done

.PHONY: lint
lint: linter
	@for dir in $$(find . -name go.mod -exec dirname {} \; ); do \
		echo "Linting $$dir"; \
		(cd $$dir && $(GO_LINT) run -v --timeout 5m ./...); \
		if [ $$? -ne 0 ]; then \
			exit 1; \
		fi; \
	done
	$(GO_LINT) run -v --timeout 5m ./...

.PHONY: check
check:
	$(GO) mod tidy > /dev/null
	@if [ ! -z "`git status -s`" ]; then \
		echo "Following files are not consistent with CI:"; \
		git status -s; \
		git diff; \
		exit 1; \
	fi

.PHONY: build
build:
	@make -C tools/go-agent-enhance build