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

package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	driver "gorm.io/driver/postgres"
)

func TestPostgresDatabaseInfoFromOpen(t *testing.T) {
	open := driver.Open("host=postgres-server user=postgres password=password dbname=test port=5432 sslmode=disable TimeZone=UTC")

	info := buildDBInfoFromDialector(open.(*driver.Dialector))
	assert.NotNil(t, info)
	assert.Equal(t, "PostgreSQL", info.Type())
	assert.Equal(t, "PostgreSQL", info.DBType())
	assert.Equal(t, "postgres-server:5432", info.Peer())
	assert.Equal(t, int32(22), info.ComponentID())
}

func TestPostgresDatabaseInfoFromNew(t *testing.T) {
	open := driver.New(driver.Config{
		DSN: "host=postgres-alt user=postgres password=password dbname=test port=5433 sslmode=disable",
	})

	info := buildDBInfoFromDialector(open.(*driver.Dialector))
	assert.NotNil(t, info)
	assert.Equal(t, "postgres-alt:5433", info.Peer())
	assert.Equal(t, int32(22), info.ComponentID())
}

func TestPostgresDatabaseInfoFromMultipleHosts(t *testing.T) {
	open := driver.New(driver.Config{
		DSN: "host=pg-1,pg-2 user=postgres password=password dbname=test port=5432,5433 sslmode=disable",
	})

	info := buildDBInfoFromDialector(open.(*driver.Dialector))
	assert.NotNil(t, info)
	assert.Equal(t, "pg-1:5432,pg-2:5433", info.Peer())
}

func TestPostgresDatabaseInfoFromNewWithSQLConn(t *testing.T) {
	open := driver.New(driver.Config{
		Conn: &testConnPool{
			field: &testSQLDatabaseInfo{peer: "postgres-from-sql:5432"},
		},
	})

	info := buildDBInfoFromDialector(open.(*driver.Dialector))
	assert.NotNil(t, info)
	assert.Equal(t, "postgres-from-sql:5432", info.Peer())
	assert.Equal(t, int32(22), info.ComponentID())
}

func TestPostgresDatabaseInfoFromDialectorValue(t *testing.T) {
	open := driver.Open("host=postgres-value user=postgres password=password dbname=test port=5434 sslmode=disable")

	info := buildDBInfoFromDialectorValue(*open.(*driver.Dialector))
	assert.NotNil(t, info)
	assert.Equal(t, "postgres-value:5434", info.Peer())
	assert.Equal(t, int32(22), info.ComponentID())
}

func TestBuildPeerAddressSpecialCases(t *testing.T) {
	cfg, err := pgx.ParseConfig("host=/var/run/postgresql user=postgres password=password dbname=test port=5432 sslmode=disable")
	assert.Nil(t, err)
	assert.Equal(t, "/var/run/postgresql/.s.PGSQL.5432", buildPeerAddress(cfg))

	cfg, err = pgx.ParseConfig("host=2001:db8::1 user=postgres password=password dbname=test port=5432 sslmode=disable")
	assert.Nil(t, err)
	assert.Equal(t, "[2001:db8::1]:5432", buildPeerAddress(cfg))
}

type testConnPool struct {
	field interface{}
}

func (i *testConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, nil
}

func (i *testConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, nil
}

func (i *testConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (i *testConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

func (i *testConnPool) GetSkyWalkingDynamicField() interface{} {
	return i.field
}

func (i *testConnPool) SetSkyWalkingDynamicField(v interface{}) {
	i.field = v
}

type testSQLDatabaseInfo struct {
	peer string
}

func (i *testSQLDatabaseInfo) DBType() string {
	return "PostgreSQL"
}

func (i *testSQLDatabaseInfo) ComponentID() int32 {
	return 22
}

func (i *testSQLDatabaseInfo) Peer() string {
	return i.peer
}
