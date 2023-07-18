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

package echov4

import (
	"fmt"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/labstack/echo/v4"
)

type Interceptor struct{}

// BeforeInvoke would be called before the target method invocation.
func (h *Interceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

// AfterInvoke would be called after the target method invocation.
func (h *Interceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	e, ok := result[0].(*echo.Echo)
	if !ok {
		return fmt.Errorf("echo :skywalking cannot create middleware for echo not match *Echo: %T", e)
	}

	e.Use(middleware())
	return nil
}

func middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			span, err := tracing.CreateEntrySpan(
				fmt.Sprintf("%s:%s", request.Method, c.Path()), func(headerKey string) (string, error) {
					return request.Header.Get(headerKey), nil
				},
				tracing.WithLayer(tracing.SpanLayerHTTP),
				tracing.WithTag(tracing.TagHTTPMethod, request.Method),
				tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path),
				tracing.WithComponent(5015))
			if err != nil {
				return err
			}

			// serve the request to the next middleware
			if err = next(c); err != nil {
				span.Error(err.Error())
				// invokes the registered HTTP error handler
				c.Error(err)
			}
			status := c.Response().Status
			if status < 200 || status > 499 {
				span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", status))
			}
			span.End()
			return err
		}
	}
}
