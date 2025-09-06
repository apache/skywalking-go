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
LINT_FILE_PATH = $(REPODIR).golangci.yml

VERSION_FILE=$(REPODIR)VERSION

SHELL = /bin/bash

HUB ?= docker.io/apache
PROJECT ?= skywalking-go

GIT_VERSION := $(shell git rev-parse --short HEAD)
ifeq ($(strip $(GIT_VERSION)),)
    GIT_VERSION = $(shell grep gitCommit $(VERSION_FILE) | awk -F ': ' '{print $$2}')
endif

VERSION ?= $(GIT_VERSION)

LOG_TARGET = echo -e "\033[0;32m===========> Running $@ ... \033[0m"

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

deps:
	$(GO_GET) -v -t -d ./...

linter:
	$(GO_LINT) version || curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GO_PATH)/bin v1.50.0

##@ General

.PHONY: generate-proto
generate-proto: ##generate data collect proto
	cd tools/protocols && ./pull-proto.sh && cd ../..

##@ Golang

.PHONY: test
test: ## Run E2E scenario tests
	@$(LOG_TARGET)
	echo "mode: atomic" > ${REPODIR}/coverage.txt;
	@for dir in $$(find . -name go.mod -exec dirname {} \; ); do \
  		if [[ $$dir == "./test/plugins/scenarios/"* ]]; then \
			continue; \
		fi; \
		cd $$dir; \
		echo "Testing $$dir"; \
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
lint: linter ## Run golangci-lint linter
	@$(LOG_TARGET)
	@for dir in $$(find . -name go.mod -exec dirname {} \; ); do \
  		if [[ $$dir == "./test/plugins/scenarios/"* ]] || [[ $$dir == "./test/e2e/"* ]]; then \
			continue; \
		fi; \
		echo "Linting $$dir"; \
		(cd $$dir && $(GO_LINT) run -v --timeout 5m --config $(LINT_FILE_PATH) ./...); \
		if [ $$? -ne 0 ]; then \
			exit 1; \
		fi; \
	done
	$(GO_LINT) run -v --timeout 5m ./...

.PHONY: check
check: ## Run consistency checks
	@$(LOG_TARGET)
	go mod tidy
	@if [ ! -z "`git status -s`" ]; then \
		echo "Following files are not consistent with CI:"; \
		git status -s; \
		git diff; \
		exit 1; \
	fi

.PHONY: version-check
version-check: ## Version-Check Check skywalking-go VERSION files
	@$(LOG_TARGET)
	@if [ ! -f $(VERSION_FILE) ]; then \
        echo "$(VERSION_FILE) file does not exist and is currently being generated"; \
        echo "version: $(VERSION)" > $(VERSION_FILE); \
        echo "gitCommit: $(GIT_VERSION)" >> $(VERSION_FILE); \
    fi

.PHONY: build
build: version-check ## Build skywalking-go agent binary
	@$(LOG_TARGET)
	@make -C tools/go-agent build

.PHONE: release
release: ## Build skywalking-go agent release
	@$(LOG_TARGET)
	/bin/sh tools/release/create_bin_release.sh
	/bin/sh tools/release/create_source_release.sh

base.all := go1.19 go1.20 go1.21 go1.22 go1.23
base.each = $(word 1, $@)


##@ Docker

base.image.go1.19 := golang:1.19
base.image.go1.20 := golang:1.20
base.image.go1.21 := golang:1.21
base.image.go1.22 := golang:1.22
base.image.go1.23 := golang:1.23

docker.%: PLATFORMS =
docker.%: LOAD_OR_PUSH = --load
docker.push.%: PLATFORMS = --platform linux/amd64,linux/arm64
docker.push.%: LOAD_OR_PUSH = --push

.PHONY: $(base.all)
$(base.all:%=docker.%): BASE_IMAGE=$($(base.each:docker.%=base.image.%))
$(base.all:%=docker.%): FINAL_TAG=$(VERSION)-$(base.each:docker.%=%)
$(base.all:%=docker.push.%): BASE_IMAGE=$($(base.each:docker.push.%=base.image.%))
$(base.all:%=docker.push.%): FINAL_TAG=$(VERSION)-$(base.each:docker.push.%=%)
$(base.all:%=docker.%) $(base.all:%=docker.push.%): version-check
	@$(version-check)
	@$(LOG_TARGET)
	docker buildx create --use --driver docker-container --name skywalking_go > /dev/null 2>&1 || true
	docker buildx build $(PLATFORMS) $(LOAD_OR_PUSH) \
        --no-cache \
        --build-arg "BASE_GO_IMAGE=$(BASE_IMAGE)" \
        --build-arg "BASE_BUILDER_IMAGE=$(BASE_IMAGE)" \
        --build-arg "VERSION=$(VERSION)" \
        . -t $(HUB)/$(PROJECT):$(FINAL_TAG)
	docker buildx rm skywalking_go || true

.PHONY: docker docker.push
docker: $(base.all:%=docker.%) ## Build docker images for skywalking-go agent and Push docker images to registry
docker.push: $(base.all:%=docker.push.%)