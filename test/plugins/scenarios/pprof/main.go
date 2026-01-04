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
	"context"
	"io"
	"log"
	"net/http"
	"runtime/pprof"
	"time"

	_ "github.com/apache/skywalking-go"
)

func providerHandler(w http.ResponseWriter, r *http.Request) {
	l := pprof.Labels("test-label", "test", "operation", "provider")
	c := context.Background()
	c = pprof.WithLabels(c, l)

	pprof.SetGoroutineLabels(c)

	doWork()
	_, _ = w.Write([]byte("success"))
}

func consumerHandler(w http.ResponseWriter, r *http.Request) {
	l := pprof.Labels("test-label", "consumer", "operation", "consumer")
	c := context.Background()
	c = pprof.WithLabels(c, l)
	pprof.SetGoroutineLabels(c)
	
	resp, err := http.Get("http://localhost:8080/provider?test=1")
	if err != nil {
		log.Printf("request provider error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	
	resp2, err := http.Get("http://localhost:8080/provider?test=2")
	if err != nil {
		log.Printf("request provider error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp2.Body.Close()
	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	
	_, _ = w.Write(append(body, body2...))
}

func doWork() {
	start := time.Now()
	for time.Since(start) < 1*time.Second {
		for i := 0; i < 1e6; i++ {
			_ = i * i
		}
	}
}

func main() {
	http.HandleFunc("/provider", providerHandler)
	http.HandleFunc("/consumer", consumerHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	_ = http.ListenAndServe("0.0.0.0:8080", nil)
}
