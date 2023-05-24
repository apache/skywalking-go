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

package core

import (
	"fmt"
	"strings"
	"time"
)

var GetGoID = func() int64 {
	return 0
}

var (
	idEpoch          time.Time
	globalInstanceID string
)

func init() {
	idEpoch = time.Date(1900, 0, 0, 0, 0, 0, 0, time.UTC)
	uuid, err := UUID()
	if err != nil {
		panic(err)
	}
	globalInstanceID = strings.ReplaceAll(uuid, "-", "")
}

type IDContext struct {
	goid     int64
	lastTime int64
	seq      int16

	shift    int64
	shitTime int64
}

func NewIDContext(getGoIDNow bool) *IDContext {
	var goid int64
	if getGoIDNow {
		goid = GetGoID()
	}
	return &IDContext{
		goid:     goid,
		lastTime: 0,
		seq:      0,
	}
}

// GenerateGlobalID generates global unique id
func GenerateGlobalID(ctx *TracingContext) (globalID string, err error) {
	idContext := ctx.ID
	if idContext.goid == 0 {
		idContext.goid = GetGoID()
	}

	return fmt.Sprintf("%s.%d.%d", globalInstanceID, idContext.goid, idContext.nextID()), nil
}

func (c *IDContext) nextID() int64 {
	return c.timestamp()*10000 + int64(c.nextSeq())
}

func (c *IDContext) timestamp() int64 {
	now := time.Since(idEpoch).Milliseconds()
	if now < c.lastTime {
		if c.shitTime != now {
			c.shift++
			c.shitTime = now
		}
		return c.shift
	}
	c.lastTime = now
	return now
}

func (c *IDContext) nextSeq() int16 {
	if c.seq == 10000 {
		c.seq = 0
	}
	c.seq++
	return c.seq
}
