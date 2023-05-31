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

package client

import (
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/selector"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type NextInterceptor struct {
}

func (n *NextInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (n *NextInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	span := tracing.ActiveSpan()
	if span != nil {
		if err, ok := results[1].(error); ok || err != nil {
			return nil
		}
		if nextSelector, ok := results[0].(selector.Next); ok && nextSelector != nil {
			var selectorWrapper selector.Next = func() (*registry.Node, error) {
				node, tmp := nextSelector()
				if node != nil {
					span.SetPeer(node.Address)
				}
				return node, tmp
			}
			invocation.DefineReturnValues(selectorWrapper, nil)
		}
	}
	return nil
}
