// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package core

import (
	"os"
	"testing"

	"github.com/apache/skywalking-go/reporter"

	"github.com/stretchr/testify/assert"
)

func TestNewEntity(t *testing.T) {
	// instance name from env
	os.Setenv("SW_AGENT_INSTANCE_ENV_NAME", "test")
	entity := NewEntity("test", "SW_AGENT_INSTANCE_ENV_NAME")
	verifyEntity(t, entity, "test", "test", false)
	// instance name from env, but env not found
	os.Setenv("SW_AGENT_INSTANCE_ENV_NAME", "")
	entity = NewEntity("test", "SW_AGENT_INSTANCE_ENV_NAME")
	verifyEntity(t, entity, "test", "", true)
	// instance env is empty
	entity = NewEntity("test", "")
	verifyEntity(t, entity, "test", "", true)
}

func TestNewTracer(t *testing.T) {
	tracer := newTracer()
	assert.NotNil(t, tracer, "tracer is nil")
	// validate all operator functions not return nil
	assert.NotNil(t, tracer.Logger(), "logger is nil")
	assert.NotNil(t, tracer.Tracing(), "tracing is nil")
}

func verifyEntity(t *testing.T, entity *reporter.Entity, serviceName, instanceName string, generatedInstance bool) {
	assert.Equal(t, serviceName, entity.ServiceName, "service name is not same")
	if generatedInstance {
		assert.Contains(t, entity.ServiceInstanceName, IPV4(), "service instance not contains the ip address")
	} else {
		assert.Equal(t, instanceName, entity.ServiceInstanceName, "service instance name is not same")
	}
	assert.NotNil(t, entity.Props, "props is nil")
	for _, p := range entity.Props {
		assert.NotNil(t, p, "prop is nil")
		assert.NotEmpty(t, p.Key, "prop key is empty")
		assert.NotEmpty(t, p.Value, "prop value is empty")
	}
}
