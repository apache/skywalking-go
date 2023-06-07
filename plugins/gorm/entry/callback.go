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
		s, err := tracing.CreateExitSpan(operation, dbInfo.Peer(), func(k, v string) error {
			return nil
		}, tracing.WithComponent(dbInfo.ComponentID()),
			tracing.WithLayer(tracing.SpanLayerDatabase),
			tracing.WithTag(tracing.TagDBType, dbInfo.Type()))

		if err != nil {
			db.Logger.Error(db.Statement.Context, "gorm:skyWalking failed to create exit span, got error: %v", err)
			return
		}

		db.Set(spanKey, s)
	}
}

func afterCallback(dbInfo DatabaseInfo) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		// get span from db instance's context
		spanInterface, _ := db.Get(spanKey)
		span, ok := spanInterface.(tracing.Span)
		if !ok {
			return
		}

		defer span.End()

		if db.Statement.Error != nil {
			span.Error(db.Statement.Error.Error())
		}
	}
}
