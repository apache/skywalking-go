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
	"github.com/jackc/pgx/v5"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

const postgreSQLComponentID int32 = 22

type OpenConnectorInterceptor struct {
}

func (i *OpenConnectorInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (i *OpenConnectorInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if tracing.GetRuntimeContextValue("needInfo") != true {
		return nil
	}
	info := i.buildDBInfo(invocation)
	if info != nil {
		tracing.SetRuntimeContextValue("info", info)
	}
	return nil
}

func (i *OpenConnectorInterceptor) buildDBInfo(invocation operator.Invocation) *DBInfo {
	if len(invocation.Args()) == 0 {
		return nil
	}
	name, ok := invocation.Args()[0].(string)
	if !ok || name == "" {
		return nil
	}
	cfg, err := pgx.ParseConfig(name)
	if err == nil {
		return buildDBInfoFromConnConfig(cfg)
	}
	return nil
}

type DBInfo struct {
	PeerAddress string
}

func buildDBInfoFromConnConfig(cfg *pgx.ConnConfig) *DBInfo {
	if cfg == nil {
		return nil
	}
	peer := buildPeerAddress(cfg)
	if peer == "" {
		return nil
	}
	return &DBInfo{PeerAddress: peer}
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

func (i *DBInfo) Peer() string {
	return i.PeerAddress
}

func (i *DBInfo) ComponentID() int32 {
	return postgreSQLComponentID
}

func (i *DBInfo) DBType() string {
	return "PostgreSQL"
}
