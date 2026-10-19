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
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"

	_ "github.com/apache/skywalking-go"
)

func main() {
	e := echo.New()
	// recover panics outside the tracing middleware, like e.Pre(middleware.Recover())
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("recovered: %v", r)
				}
			}()
			return next(c)
		}
	})
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

		// the span of a panicking handler must still be ended and reported
		panicResp, err := http.Get("http://localhost:8080/panic")
		if err != nil {
			log.Print(err)
			c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return err
		}
		_, _ = io.Copy(io.Discard, panicResp.Body)
		_ = panicResp.Body.Close()

		// a propagation header that cannot be decoded must not fail the request
		if err = requestWithMalformedSW8("localhost:8080", "/provider"); err != nil {
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

	e.GET("/panic", func(c echo.Context) error {
		panic("boom")
	})

	e.GET("/health", func(c echo.Context) error {
		c.String(http.StatusOK, "")
		return nil
	})

	_ = e.Start(":8080")
}

// requestWithMalformedSW8 sends a request whose sw8 header cannot be decoded and
// expects it to succeed. It writes the request on a plain connection, because the
// instrumented http.Transport would add a valid sw8 header of its own.
func requestWithMalformedSW8(host, path string) error {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		return err
	}
	defer conn.Close()
	request, err := http.NewRequest(http.MethodGet, "http://"+host+path, http.NoBody)
	if err != nil {
		return err
	}
	request.Header.Set("sw8", "x")
	request.Close = true
	if err = request.Write(conn); err != nil {
		return err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request with a malformed sw8 header failed: %d", resp.StatusCode)
	}
	return nil
}
