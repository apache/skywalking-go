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
	"net/http"
	"sync"
	"time"

	_ "github.com/apache/skywalking-go"
	"github.com/apache/skywalking-go/toolkit/trace"
)

func executeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request for /execute")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		trace.CreateLocalSpan("testGoroutineLocalSpan")
		time.Sleep(100 * time.Millisecond)
		trace.StopSpan()
	}()
	wg.Wait()
	log.Println("Goroutine finished, sending response")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func main() {
	http.HandleFunc("/execute", executeHandler)
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	_ = http.ListenAndServe(":8080", nil)
}
