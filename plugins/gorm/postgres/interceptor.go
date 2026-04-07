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
	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const postgreSQLComponentID int32 = 22
const postgreSQLDBType = "PostgreSQL"

type sqlDatabaseInfo interface {
	DBType() string
	ComponentID() int32
	Peer() string
}

func buildDBInfoFromDialector(dial *postgres.Dialector) *DatabaseInfo {
	if dial == nil {
		return nil
	}
	return buildDBInfoFromConfig(dial.Config)
}

func buildDBInfoFromDialectorValue(dial postgres.Dialector) *DatabaseInfo {
	return buildDBInfoFromConfig(dial.Config)
}

func buildDBInfoFromConfig(config *postgres.Config) *DatabaseInfo {
	if config == nil {
		return nil
	}
	if config.DSN != "" {
		cfg, err := pgx.ParseConfig(config.DSN)
		if err == nil {
			peer := buildPeerAddress(cfg)
			if peer != "" {
				return &DatabaseInfo{PeerAddress: peer}
			}
		}
	}
	return buildDBInfoFromConn(config.Conn)
}

type DatabaseInfo struct {
	PeerAddress string
}

func (d *DatabaseInfo) Type() string {
	return postgreSQLDBType
}

func (d *DatabaseInfo) DBType() string {
	return postgreSQLDBType
}

func (d *DatabaseInfo) ComponentID() int32 {
	return postgreSQLComponentID
}

func (d *DatabaseInfo) Peer() string {
	return d.PeerAddress
}

func buildPeerAddress(cfg *pgx.ConnConfig) string {
	if cfg == nil {
		return ""
	}
	fallbacks := make([]tracing.PostgreSQLAddress, 0, len(cfg.Fallbacks))
	for _, fallback := range cfg.Fallbacks {
		if fallback == nil {
			continue
		}
		fallbacks = append(fallbacks, tracing.PostgreSQLAddress{Host: fallback.Host, Port: fallback.Port})
	}
	return tracing.BuildPostgreSQLPeer(
		tracing.PostgreSQLAddress{Host: cfg.Host, Port: cfg.Port},
		fallbacks,
	)
}

func buildDBInfoFromConn(conn interface{}) *DatabaseInfo {
	ins, ok := conn.(operator.EnhancedInstance)
	if !ok || ins == nil {
		return nil
	}
	return adaptSQLDatabaseInfo(ins.GetSkyWalkingDynamicField())
}

func adaptSQLDatabaseInfo(v interface{}) *DatabaseInfo {
	switch info := v.(type) {
	case nil:
		return nil
	case *DatabaseInfo:
		return info
	case sqlDatabaseInfo:
		if info.DBType() != postgreSQLDBType || info.Peer() == "" {
			return nil
		}
		return &DatabaseInfo{PeerAddress: info.Peer()}
	default:
		return nil
	}
}

type InstanceInterceptor struct {
}

func (i *InstanceInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (i *InstanceInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if res, ok := result[0].(*postgres.Dialector); ok && res != nil {
		dbInfo := buildDBInfoFromDialector(res)
		if caller, ok := result[0].(operator.EnhancedInstance); ok && dbInfo != nil {
			caller.SetSkyWalkingDynamicField(dbInfo)
		}
	}
	return nil
}

type InitializeInterceptor struct {
}

func (i *InitializeInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (i *InitializeInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if err, ok := result[0].(error); ok && err != nil {
		return nil
	}
	db, ok := invocation.Args()[0].(*gorm.DB)
	if !ok || db == nil || db.ConnPool == nil {
		return nil
	}
	connPool, ok := db.ConnPool.(operator.EnhancedInstance)
	if !ok || connPool == nil {
		return nil
	}
	if connPool.GetSkyWalkingDynamicField() != nil {
		return nil
	}
	dbInfo := buildDBInfoFromInvocation(invocation)
	if dbInfo == nil {
		return nil
	}
	connPool.SetSkyWalkingDynamicField(dbInfo)
	return nil
}

func buildDBInfoFromInvocation(invocation operator.Invocation) *DatabaseInfo {
	if invocation == nil {
		return nil
	}
	if caller, ok := invocation.CallerInstance().(operator.EnhancedInstance); ok && caller != nil {
		if dbInfo := adaptSQLDatabaseInfo(caller.GetSkyWalkingDynamicField()); dbInfo != nil {
			return dbInfo
		}
	}
	switch caller := invocation.CallerInstance().(type) {
	case *postgres.Dialector:
		return buildDBInfoFromDialector(caller)
	case postgres.Dialector:
		return buildDBInfoFromDialectorValue(caller)
	default:
		return nil
	}
}
