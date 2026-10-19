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

package pulsar

import (
	"github.com/apache/pulsar-client-go/pulsar"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

// NewClientInterceptor saves the service URL of a new client on the client.
// The producer and consumer spans use it as their peer and broker, as the
// Java agent does, so that no interceptor needs a broker lookup RPC.
type NewClientInterceptor struct {
}

func (n *NewClientInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (n *NewClientInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	options, ok := invocation.Args()[0].(pulsar.ClientOptions)
	if !ok {
		return nil
	}
	// a failed NewClient returns a nil Client
	if instance, ok := result[0].(operator.EnhancedInstance); ok {
		instance.SetSkyWalkingDynamicField(options.URL)
	}
	return nil
}

// serviceURL returns the service URL that NewClientInterceptor saved on the
// client, or "" when there is none. It does no network I/O.
func serviceURL(c *nativeclient) string {
	if c == nil {
		return ""
	}
	if instance, ok := interface{}(c).(operator.EnhancedInstance); ok {
		if url, ok := instance.GetSkyWalkingDynamicField().(string); ok {
			return url
		}
	}
	return ""
}
