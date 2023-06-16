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

package mysql

import (
	"github.com/go-sql-driver/mysql"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type ParseInterceptor struct {
}

func (n *ParseInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (n *ParseInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	if cfg, ok := results[0].(*mysql.Config); ok && cfg != nil && tracing.GetRuntimeContextValue("needInfo") == true {
		tracing.SetRuntimeContextValue("info", &DBInfo{Addr: cfg.Addr})
	}
	return nil
}

type DBInfo struct {
	Addr string
}

func (i *DBInfo) Peer() string {
	return i.Addr
}

func (i *DBInfo) ComponentID() int32 {
	return 5012
}

func (i *DBInfo) DBType() string {
	return "Mysql"
}
