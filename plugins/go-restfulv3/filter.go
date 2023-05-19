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

package restfulv3

import (
	"fmt"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"

	restful "github.com/emicklei/go-restful/v3"
)

var filterInstance = func(request *restful.Request, response *restful.Response, chain *restful.FilterChain) {
	s, err := tracing.CreateEntrySpan(request.Request.Method+":"+request.SelectedRoutePath(), func(k string) (string, error) {
		return request.HeaderParameter(k), nil
	}, tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, request.Request.Method),
		tracing.WithTag(tracing.TagURL, request.Request.Host+request.Request.URL.Path),
		tracing.WithComponent(5004))
	if err != nil {
		chain.ProcessFilter(request, response)
		return
	}
	defer func() {
		code := response.StatusCode()
		if response.Error() != nil {
			s.Error(response.Error().Error())
		}
		s.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", code))
		s.End()
	}()
	chain.ProcessFilter(request, response)
}

func addFilterToContainer(c interface{}) {
	if instance, ok := c.(operator.EnhancedInstance); ok && instance.GetSkyWalkingDynamicField() == nil {
		instance.SetSkyWalkingDynamicField(true)
	} else {
		return
	}

	if container, ok := c.(*restful.Container); ok {
		container.Filter(filterInstance)
	}
}
