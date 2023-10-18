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
	"log"
	"time"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	_ "github.com/apache/skywalking-go"
)

func providerHandler(ctx *fasthttp.RequestCtx) {
	ctx.WriteString("success")
}

func consumerHandler(ctx *fasthttp.RequestCtx) {
	url := "http://localhost:8080/provider"

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)

	resp := fasthttp.AcquireResponse()

	timeout := 5 * time.Second
	var defaultClient fasthttp.Client
	err := defaultClient.DoTimeout(req, resp, timeout)

	if err != nil {
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(resp.StatusCode())
	_, err = ctx.Write(resp.Body())
	if err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(resp)
}

func healthHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
}

func main() {
	r := router.New()

	r.GET("/provider", providerHandler)
	r.GET("/consumer", consumerHandler)
	r.GET("/health", healthHandler)

	err := fasthttp.ListenAndServe(":8080", r.Handler)
	if err != nil {
		log.Print(err)
		return
	}
}
