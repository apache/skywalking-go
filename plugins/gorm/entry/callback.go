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
	"fmt"

	"gorm.io/gorm"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

var spanKey = "skywalking-span"

func beforeCallback(dbInfo DatabaseInfo, op string) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		tableName := db.Statement.Table
		operation := fmt.Sprintf("%s/%s", tableName, op)
		// a leftover span on this very Statement means a chained *gorm.DB is
		// shared across goroutines (unsupported by gorm): the previous span is
		// about to be overwritten and lost, so make the misuse visible
		if leftover, ok := db.InstanceGet(spanKey); ok {
			if _, isSpan := leftover.(tracing.Span); isSpan {
				db.Logger.Warn(db.Statement.Context,
					"gorm:skywalking found an unfinished span on the statement, "+
						"the *gorm.DB is probably shared across goroutines; its trace data will be lost")
			}
		}
		s, err := tracing.CreateExitSpan(operation, dbInfo.Peer(), func(k, v string) error {
			return nil
		}, tracing.WithComponent(dbInfo.ComponentID()),
			tracing.WithLayer(tracing.SpanLayerDatabase),
			tracing.WithTag(tracing.TagDBType, dbInfo.Type()))

		if err != nil {
			db.Logger.Error(db.Statement.Context, "gorm:skyWalking failed to create exit span, got error: %v", err)
			return
		}

		// InstanceSet keys by the Statement pointer: gorm's Statement.clone
		// copies plain db.Set Settings into every Session/Transaction clone,
		// which let a derived operation pick up - and end - the OUTER span
		db.InstanceSet(spanKey, s)
	}
}

func afterCallback(dbInfo DatabaseInfo) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		// get span from db instance's context
		spanInterface, _ := db.InstanceGet(spanKey)
		span, ok := spanInterface.(tracing.Span)
		if !ok {
			return
		}
		// the span is consumed: a later operation on the same statement must
		// not see it as a leftover
		db.InstanceSet(spanKey, nil)

		defer span.End()

		span.Tag(tracing.TagDBStatement, db.Statement.SQL.String())
		if config.CollectParameter && len(db.Statement.Vars) > 0 {
			span.Tag(tracing.TagDBSqlParameters, argsToString(db.Statement.Vars))
		}
		if db.Statement.Error != nil {
			span.Error(db.Statement.Error.Error())
		}
	}
}

func argsToString(args []interface{}) string {
	switch len(args) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%v", args[0])
	}

	res := fmt.Sprintf("%v", args[0])
	for _, arg := range args[1:] {
		res += fmt.Sprintf(", %v", arg)
	}
	return res
}
