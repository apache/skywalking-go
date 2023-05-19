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

package main

import (
	"io"
	"net/http"

	restful "github.com/emicklei/go-restful/v3"

	_ "github.com/apache/skywalking-go"
)

func main() {
	ws := new(restful.WebService)

	ws.Route(ws.GET("/health").To(func(request *restful.Request, response *restful.Response) {
		_, _ = response.Write([]byte("success"))
	}).Doc("health checker"))

	ws.Route(ws.GET("/consumer").To(func(request *restful.Request, response *restful.Response) {
		resp, err := http.Get("http://localhost:8080/provider/1")
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = response.Write(body)
	}))

	ws.Route(ws.GET("/provider/{user}").To(func(request *restful.Request, response *restful.Response) {
		_, _ = response.Write([]byte("success"))
	}).Doc("provider"))

	restful.Add(ws)

	http.ListenAndServe(":8080", nil)
}
