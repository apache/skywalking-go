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
	"gorm.io/gorm"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

type OpenInterceptor struct {
}

func (i *OpenInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

// AfterInvoke would be called after the target method invocation.
func (i *OpenInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if e, ok := result[1].(error); ok && e != nil {
		return nil
	}
	db, ok := result[0].(*gorm.DB)
	if !ok || db == nil {
		return nil
	}

	// setup database info
	info := i.setupDatabaseInfo(db)
	if info == nil {
		return nil
	}

	// add the callback
	_ = db.Callback().Create().Before("gorm:create").Register("sky_create_create_span", beforeCallback(info, "create"))
	_ = db.Callback().Query().Before("gorm:query").Register("sky_create_query_span", beforeCallback(info, "query"))
	_ = db.Callback().Update().Before("gorm:update").Register("sky_create_update_span", beforeCallback(info, "update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("sky_create_delete_span", beforeCallback(info, "delete"))
	_ = db.Callback().Row().Before("gorm:row").Register("sky_create_row_span", beforeCallback(info, "row"))
	_ = db.Callback().Raw().Before("gorm:raw").Register("sky_create_raw_span", beforeCallback(info, "raw"))

	// after database operation
	_ = db.Callback().Create().After("gorm:create").Register("sky_end_create_span", afterCallback(info))
	_ = db.Callback().Query().After("gorm:query").Register("sky_end_query_span", afterCallback(info))
	_ = db.Callback().Update().After("gorm:update").Register("sky_end_update_span", afterCallback(info))
	_ = db.Callback().Delete().After("gorm:delete").Register("sky_end_delete_span", afterCallback(info))
	_ = db.Callback().Row().After("gorm:row").Register("sky_end_row_span", afterCallback(info))
	_ = db.Callback().Raw().After("gorm:raw").Register("sky_end_raw_span", afterCallback(info))

	return nil
}

func (i *OpenInterceptor) setupDatabaseInfo(db *gorm.DB) DatabaseInfo {
	if db.Config == nil || db.Config.Dialector == nil {
		return nil
	}
	ins, ok := db.Config.Dialector.(operator.EnhancedInstance)
	if !ok {
		return nil
	}
	dbInfo, ok := ins.GetSkyWalkingDynamicField().(DatabaseInfo)
	if !ok || dbInfo == nil {
		return nil
	}

	return dbInfo
}
