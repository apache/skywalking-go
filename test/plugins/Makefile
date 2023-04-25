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

GO = go
GO_BUILD = $(GO) build

.PHONY: build
build:
	mkdir -p dist
	# go agent
	@make -C ../../tools/go-agent linux
	cp ../../bin/skywalking-go-agent--linux-amd64 dist/skywalking-go-agent
	# runner helper
	cd runner-helper && $(GO_BUILD) -o ../dist/runner-helper
	# validator container
	cd validator && docker build . -t skywalking/agent-test-validator:1.0.0
	docker save -o dist/skywalking-agent-test-validator-1.0.0.tgz skywalking/agent-test-validator:1.0.0