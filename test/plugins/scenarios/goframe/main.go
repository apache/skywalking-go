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

//go:nolint
import (
	"context"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/apache/skywalking-go"
)

//go:nolint
func main() {
	command := gcmd.Command{
		Name:  "goframe",
		Usage: "goframe",
		Brief: "start http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server()
			s.SetPort(8080)
			s.BindHandler("/provider", func(r *ghttp.Request) {
				r.Response.Write("success")
			})
			s.BindHandler("/consumer", func(r *ghttp.Request) {
				client := g.Client()
				client.SetHeader("h1", "h1")
				client.SetHeader("h2", "h2")
				var resp, err = client.Get(gctx.GetInitCtx(), "http://localhost:8080/provider?test=1")
				if err != nil {
					r.Response.Write(err.Error())
					return
				}
				defer resp.Close()
				var str = resp.ReadAllString()
				r.Response.Write(str)
			})
			s.BindHandler("/health", func(r *ghttp.Request) {
				r.Response.WriteHeader(http.StatusOK)
				r.Response.Write("success")
			})
			s.Run()
			return nil
		},
	}
	command.Run(gctx.New())

}
