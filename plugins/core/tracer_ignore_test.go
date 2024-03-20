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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceIgnorePath(t *testing.T) {
	ignorePath := []string{"/health/*"}
	assert.True(t, traceIgnorePath("GET:/health/apps", ignorePath))
	assert.True(t, traceIgnorePath("GET:/health/", ignorePath))
	assert.True(t, traceIgnorePath("GET:/health/apps/", ignorePath))

	ignorePath = []string{"/health/**"}
	assert.True(t, traceIgnorePath("GET:/health/apps/", ignorePath))

	ignorePath = []string{"/health/*/"}
	assert.True(t, traceIgnorePath("GET:/health/apps/", ignorePath))
	assert.False(t, traceIgnorePath("GET:/health/", ignorePath))
	assert.False(t, traceIgnorePath("GET:/health/apps/list", ignorePath))
	assert.False(t, traceIgnorePath("GET:/health/test", ignorePath))

	ignorePath = []string{"/health/**"}
	assert.True(t, traceIgnorePath("GET:/health/", ignorePath))
	assert.True(t, traceIgnorePath("GET:/health/apps/test", ignorePath))
	assert.True(t, traceIgnorePath("GET:/health/apps/test/", ignorePath))

	ignorePath = []string{"health/apps/?"}
	assert.False(t, traceIgnorePath("GET:health/apps/list", ignorePath))
	assert.False(t, traceIgnorePath("GET:health/apps/", ignorePath))
	assert.True(t, traceIgnorePath("GET:health/apps/a", ignorePath))

	ignorePath = []string{"health/**/lists"}
	assert.True(t, traceIgnorePath("GET:health/apps/lists", ignorePath))
	assert.True(t, traceIgnorePath("GET:health/apps/test/lists", ignorePath))
	assert.False(t, traceIgnorePath("GET:health/apps/test/", ignorePath))
	assert.False(t, traceIgnorePath("GET:health/apps/test", ignorePath))

	ignorePath = []string{"health/**/test/**"}
	assert.True(t, traceIgnorePath("GET:health/apps/test/list", ignorePath))
	assert.True(t, traceIgnorePath("GET:health/apps/foo/test/list/bar", ignorePath))
	assert.True(t, traceIgnorePath("GET:health/apps/foo/test/list/bar/", ignorePath))
	assert.True(t, traceIgnorePath("GET:health/apps/test/list", ignorePath))
	assert.True(t, traceIgnorePath("GET:health/test/list", ignorePath))

	ignorePath = []string{"/health/**/b/**/*.txt", "abc/*"}
	assert.True(t, traceIgnorePath("GET:/health/a/aa/aaa/b/bb/bbb/xxxxxx.txt", ignorePath))
	assert.False(t, traceIgnorePath("GET:/health/a/aa/aaa/b/bb/bbb/xxxxxx", ignorePath))
	assert.False(t, traceIgnorePath("GET:abc/foo/bar", ignorePath))
	assert.True(t, traceIgnorePath("GET:abc/foo", ignorePath))
}
