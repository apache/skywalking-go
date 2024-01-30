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

package amqp

import (
	"fmt"
	"net/url"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

type DialInterceptor struct {
}

func (d *DialInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (d *DialInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if instance, ok := result[0].(*amqp.Connection); ok && instance != nil {
		address := parseURI(invocation.Args()[0].(string))
		result[0].(operator.EnhancedInstance).SetSkyWalkingDynamicField(address)
	}
	return nil
}

func parseURI(uri string) string {
	var ret = ""
	if strings.Contains(uri, " ") {
		return ret
	}
	u, err := url.Parse(uri)
	if err != nil {
		return ret
	}
	return fmt.Sprintf("%s:%s", u.Hostname(), u.Port())
}

func getPeerInfo(filed interface{}) string {
	instance, ok := filed.(operator.EnhancedInstance)
	if !ok || instance == nil {
		return ""
	}
	return instance.GetSkyWalkingDynamicField().(string)
}
