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

package plugins

import (
	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/plugins/dubbo"
	"github.com/apache/skywalking-go/plugins/gin"
	goredisv9 "github.com/apache/skywalking-go/plugins/go-redisv9"
	"github.com/apache/skywalking-go/plugins/go-restfulv3"
	gorm_entry "github.com/apache/skywalking-go/plugins/gorm/entry"
	gorm_mysql "github.com/apache/skywalking-go/plugins/gorm/mysql"
	"github.com/apache/skywalking-go/plugins/http"
	"github.com/apache/skywalking-go/plugins/kratosv2"
	"github.com/apache/skywalking-go/plugins/microv4"
	"github.com/apache/skywalking-go/plugins/sarama"
)

var instruments = make([]instrument.Instrument, 0)

func init() {
	// register the plugins instrument
	registerFramework(gin.NewInstrument())
	registerFramework(http.NewInstrument())
	registerFramework(dubbo.NewInstrument())
	registerFramework(restfulv3.NewInstrument())
	registerFramework(kratosv2.NewInstrument())
	registerFramework(microv4.NewInstrument())
	registerFramework(goredisv9.NewInstrument())
	registerFramework(sarama.NewInstrument())

	// gorm related instruments
	registerFramework(gorm_entry.NewInstrument())
	registerFramework(gorm_mysql.NewInstrument())
}

func registerFramework(ins instrument.Instrument) {
	instruments = append(instruments, ins)
}
