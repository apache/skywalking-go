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

	"github.com/labstack/echo/v4"

	_ "github.com/apache/skywalking-go"
)

func main() {
	e := echo.New()
	e.GET("/consumer", func(c echo.Context) error {
		resp, err := http.Get("http://localhost:8080/provider")
		if err != nil {
			log.Print(err)
			c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Print(err)
			c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return err
		}
		c.String(http.StatusOK, string(body))
		return nil
	})

	e.GET("/provider", func(c echo.Context) error {
		c.String(http.StatusOK, "success")
		return nil
	})

	e.GET("/health", func(c echo.Context) error {
		c.String(http.StatusOK, "")
		return nil
	})

	_ = e.Start(":8080")
}
