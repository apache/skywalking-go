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

	"github.com/gofiber/fiber/v2"

	_ "github.com/apache/skywalking-go"
)

func main() {
	app := fiber.New()
	app.Get("/consumer", func(ctx *fiber.Ctx) error {
		resp, err := http.Get("http://localhost:8080/provider")
		if err != nil {
			log.Print(err)
			ctx.Status(http.StatusInternalServerError)
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Print(err)
			ctx.Status(http.StatusInternalServerError)
			return err
		}
		ctx.WriteString(string(body))
		return nil
	})

	app.Get("/provider", func(ctx *fiber.Ctx) error {
		ctx.WriteString("success")
		return nil
	})

	app.Get("health", func(ctx *fiber.Ctx) error {
		ctx.Status(http.StatusOK)
		return nil
	})

	_ = app.Listen(":8080")
}