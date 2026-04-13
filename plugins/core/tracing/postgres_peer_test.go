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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPostgreSQLPeer(t *testing.T) {
	peer := BuildPostgreSQLPeer(
		PostgreSQLAddress{Host: "pg-1", Port: 5432},
		[]PostgreSQLAddress{{Host: "pg-2", Port: 5433}},
	)
	assert.Equal(t, "pg-1:5432,pg-2:5433", peer)
}

func TestBuildPostgreSQLPeerUnixSocket(t *testing.T) {
	peer := BuildPostgreSQLPeer(PostgreSQLAddress{Host: "/var/run/postgresql", Port: 5432}, nil)
	assert.Equal(t, "/var/run/postgresql/.s.PGSQL.5432", peer)
}

func TestBuildPostgreSQLPeerIPv6(t *testing.T) {
	peer := BuildPostgreSQLPeer(PostgreSQLAddress{Host: "2001:db8::1", Port: 5432}, nil)
	assert.Equal(t, "[2001:db8::1]:5432", peer)
}

func TestBuildPostgreSQLPeerDeduplicate(t *testing.T) {
	peer := BuildPostgreSQLPeer(
		PostgreSQLAddress{Host: "pg-1", Port: 5432},
		[]PostgreSQLAddress{{Host: "pg-1", Port: 5432}},
	)
	assert.Equal(t, "pg-1:5432", peer)
}
