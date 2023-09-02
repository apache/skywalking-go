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
	"log"
	"net/http"

	"github.com/kataras/iris/v12"

	_ "github.com/apache/skywalking-go"
)

func main() {
	app := iris.New()
	app.Use(iris.Compression)
	app.Get("/consumer", func(context iris.Context) {
		resp, err := http.Get("http://localhost:8080/provider")
		if err != nil {
			log.Print(err)
			context.StatusCode(http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Print(err)
			context.StatusCode(http.StatusInternalServerError)
			return
		}
		context.WriteString(string(body))
	})

	app.Get("/provider", func(context iris.Context) {
		context.Text("success")
	})

	app.Get("health", func(context iris.Context) {
		context.StatusCode(http.StatusOK)
	})

	_ = app.Run(iris.Addr(":8080"))
}
