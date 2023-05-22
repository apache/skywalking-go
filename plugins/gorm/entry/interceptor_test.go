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

package entry

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/plugins/gorm/mysql"

	"github.com/stretchr/testify/assert"

	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

func init() {
	core.ResetTracingContext()
}

var errConnectionExecute = fmt.Errorf("test error")

const peerAddress = "localhost:3306"

func TestInterceptor(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &OpenInterceptor{}
	db, err := gorm.Open(NewTestDialector(&mysql.DatabaseInfo{PeerAddress: peerAddress}))
	assert.Nil(t, err, "failed to open database")
	assert.NotNil(t, db, "failed to open database")
	err = interceptor.AfterInvoke(nil, db, err)
	assert.Nil(t, err, "failed to invoke AfterInvoke")

	res := db.Exec("select * from test")

	assert.Equal(t, errConnectionExecute, res.Error, "failed to invoke Rows")
	time.Sleep(100 * time.Millisecond)
	spans := core.GetReportedSpans()
	assert.NotNil(t, spans, "spans should not be nil")
	assert.Equal(t, 1, len(spans), "spans length should be 1")
	assert.Equal(t, peerAddress, spans[0].Peer(), "peer should be localhost:3306")
	assert.Equal(t, int32(5012), spans[0].ComponentID(), "component id should be 5012")
	assert.Equal(t, "/raw", spans[0].OperationName(), "operation name should be /raw")
	assert.Nil(t, spans[0].Refs(), "refs should be nil")
	assert.Greater(t, spans[0].StartTime(), int64(0), "end time should be greater than zero")
	assert.Greater(t, spans[0].EndTime(), int64(0), "end time should be greater than zero")
}

type TestDialector struct {
	gorm.Dialector
	field interface{}
}

func NewTestDialector(v interface{}) *TestDialector {
	return &TestDialector{Dialector: &tests.DummyDialector{}, field: v}
}

func (i *TestDialector) Initialize(db *gorm.DB) error {
	_ = i.Dialector.Initialize(db)
	db.ConnPool = &TestConnPool{}
	return nil
}

func (i *TestDialector) GetSkyWalkingDynamicField() interface{} {
	return i.field
}

func (i *TestDialector) SetSkyWalkingDynamicField(v interface{}) {
	i.field = v
}

type TestConnPool struct {
}

func (i *TestConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, errConnectionExecute
}

func (i *TestConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, errConnectionExecute
}

func (i *TestConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, errConnectionExecute
}

func (i *TestConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}
