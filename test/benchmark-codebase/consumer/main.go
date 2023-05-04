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
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	_ "github.com/apache/skywalking-go"
)

var providerAddress = flag.String("provider", "", "provider address")

var netTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 2 * time.Minute,
	}).DialContext,
	MaxIdleConns:          0,
	MaxIdleConnsPerHost:   math.MaxInt,
	IdleConnTimeout:       90 * time.Second,
	ExpectContinueTimeout: 10 * time.Second,
}

var customClient = http.Client{
	Timeout:   time.Second * 10,
	Transport: netTransport,
}

const rpcCount = 3

func main() {
	flag.Parse()
	if *providerAddress == "" {
		panic("provider address is empty")
	}
	startPprof()
	engine := gin.New()
	engine.Handle("GET", "/consumer", func(context *gin.Context) {
		for i := 0; i < rpcCount; i++ {
			v := mockBusinessCode()
			resp, err := customClient.Get(fmt.Sprintf("%s?v=%f", *providerAddress, v))
			if err != nil {
				log.Printf("send provider request error: %v", err)
				_ = context.AbortWithError(500, err)
				return
			}
			_, err = io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			_ = resp.Body.Close()
		}
		context.String(200, "success")
	})

	engine.Handle("GET", "/kill", func(context *gin.Context) {
		os.Exit(0)
	})
	_ = engine.Run(":8080")
}

func mockBusinessCode() float64 {
	iterations := 35_000

	var res float64
	for i := 0; i < iterations; i++ {
		res += math.Log(math.MaxFloat64 - float64(i))
	}
	return res
}

func startPprof() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	svr := &http.Server{
		Addr:              ":6060",
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           mux,
	}
	go func() {
		if err := svr.ListenAndServe(); err != nil {
			log.Printf("starting pprof server failure: %v", err)
		}
	}()
}
