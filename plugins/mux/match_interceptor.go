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

package mux

import (
	"net/http"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type MatchInterceptor struct {
}

func (n *MatchInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

func (n *MatchInterceptor) AfterInvoke(invocation operator.Invocation, results ...interface{}) error {
	// only process with matched route
	if matched, ok := results[0].(bool); !ok || !matched {
		return nil
	}
	match, ok := invocation.Args()[1].(*RouteMatch)
	req := invocation.Args()[0].(*http.Request)
	if !ok || match == nil || match.Route == nil || req == nil {
		return nil
	}

	span := tracing.ActiveSpan()
	if span == nil {
		return nil
	}

	// find matched template
	var routePrefix, routePath string
	for _, matcher := range match.Route.matchers {
		if regexp, ok := matcher.(*routeRegexp); ok && regexp != nil {
			if regexp.regexpType == 2 {
				routePrefix = regexp.template
			} else if regexp.regexpType == 0 {
				routePath = regexp.template
			}
		}
	}

	opName := routePrefix
	if routePath != "" {
		opName = routePath
	}

	// re-set the operation name if route path/prefix not empty
	if opName != "" {
		span.SetOperationName(req.Method + ":" + opName)
	}

	return nil
}
