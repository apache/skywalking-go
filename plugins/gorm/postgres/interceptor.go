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
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"

	"github.com/apache/skywalking-go/plugins/core/operator"
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
	addresses := make([]string, 0, len(cfg.Fallbacks)+1)
	addresses = appendPeerAddress(addresses, cfg.Host, cfg.Port)
	for _, fallback := range cfg.Fallbacks {
		if fallback == nil {
			continue
		}
		addresses = appendPeerAddress(addresses, fallback.Host, fallback.Port)
	}
	return strings.Join(addresses, ",")
}

func appendPeerAddress(addresses []string, host string, port uint16) []string {
	if host == "" {
		return addresses
	}
	address := host + ":" + strconv.Itoa(int(port))
	if strings.HasPrefix(host, "/") {
		if strings.HasSuffix(host, "/") {
			address = host + ".s.PGSQL." + strconv.Itoa(int(port))
		} else {
			address = host + "/.s.PGSQL." + strconv.Itoa(int(port))
		}
	} else if strings.Count(host, ":") > 1 && !strings.HasPrefix(host, "[") {
		address = "[" + host + "]:" + strconv.Itoa(int(port))
	}
	for _, existed := range addresses {
		if existed == address {
			return addresses
		}
	}
	return append(addresses, address)
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
