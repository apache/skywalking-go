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

package server

import (
	"context"

	"go-micro.dev/v4/server"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

//skywalking:public
func NewServerWrapper(fn server.HandlerFunc) server.HandlerFunc {
	return func(ctx context.Context, req server.Request, rsp interface{}) error {
		// the entry span should be some other frameworks, such as http.
		span, err := tracing.CreateLocalSpan(req.Service()+"."+req.Endpoint(),
			tracing.WithComponent(5009),
			tracing.WithLayer(tracing.SpanLayerRPCFramework))
		if err != nil {
			return err
		}

		defer span.End()
		if err = fn(ctx, req, rsp); err != nil {
			span.Error(err.Error())
		}
		return err
	}
}
