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

package goelasticsearchv8

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/go-elasticsearch/v8"
)

type ESV8Interceptor struct {
}

func (es *ESV8Interceptor) BeforeInvoke(invocation operator.Invocation) error {
	client := invocation.CallerInstance().(*elasticsearch.BaseClient)
	var addresses []string
	for _, u := range client.Transport.(*elastictransport.Client).URLs() {
		addresses = append(addresses, u.String())
	}
	url := strings.Join(addresses, ",")
	req := invocation.Args()[0].(*http.Request)
	span, err := tracing.CreateExitSpan("Elasticsearch/"+req.Method, url, func(headerKey, headerValue string) error {
		req.Header.Add(headerKey, headerValue)
		return nil
	},
		tracing.WithLayer(tracing.SpanLayerDatabase),
		tracing.WithTag(tracing.TagDBType, "Elasticsearch"),
		tracing.WithTag(tracing.TagDBStatement, strings.TrimPrefix(req.URL.Path, "/")),
		tracing.WithComponent(47),
	)
	if err != nil {
		return err
	}
	invocation.SetContext(span)
	return nil
}

func (es *ESV8Interceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	if resp, ok := result[0].(*http.Response); ok && resp != nil {
		span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", resp.StatusCode))
	}
	if err, ok := result[1].(error); ok && err != nil {
		span.Error(err.Error())
	}
	span.End()
	return nil
}
