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

package pgxstdlib

import (
	"testing"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestOpenConnectorBuildDBInfoFromDSN(t *testing.T) {
	core.ResetTracingContext()
	defer core.ResetTracingContext()

	tracing.SetRuntimeContextValue(tracing.SQLNeedInfoRuntimeContextKey, true)

	interceptor := &OpenConnectorInterceptor{}
	dsn := "postgres://user:password@postgres:5432/database?sslmode=disable"
	err := interceptor.AfterInvoke(operator.NewInvocation(nil, dsn), nil, nil)
	assert.Nil(t, err)

	info, ok := tracing.GetRuntimeContextValue(tracing.SQLInfoRuntimeContextKey).(*DBInfo)
	assert.True(t, ok)
	assert.Equal(t, "postgres:5432", info.Peer())
	assert.Equal(t, int32(22), info.ComponentID())
	assert.Equal(t, "PostgreSQL", info.DBType())
}

func TestBuildPeerAddressSpecialCases(t *testing.T) {
	cfg, err := pgx.ParseConfig("host=pg-1,pg-2 user=user password=password dbname=test port=5432,5433 sslmode=disable")
	assert.Nil(t, err)
	assert.Equal(t, "pg-1:5432,pg-2:5433", buildPeerAddress(cfg))

	cfg, err = pgx.ParseConfig("host=/var/run/postgresql user=user password=password dbname=test port=5432 sslmode=disable")
	assert.Nil(t, err)
	assert.Equal(t, "/var/run/postgresql/.s.PGSQL.5432", buildPeerAddress(cfg))

	cfg, err = pgx.ParseConfig("host=2001:db8::1 user=user password=password dbname=test port=5432 sslmode=disable")
	assert.Nil(t, err)
	assert.Equal(t, "[2001:db8::1]:5432", buildPeerAddress(cfg))
}

func TestOpenConnectorBuildDBInfoFromUnsupportedRegisteredConnToken(t *testing.T) {
	info := (&OpenConnectorInterceptor{}).buildDBInfo(operator.NewInvocation(nil, "registeredConnConfig0"))
	assert.Nil(t, info)
}
