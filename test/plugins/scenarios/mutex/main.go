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
	"net/http"
	"sync"
	"time"

	_ "github.com/apache/skywalking-go"
)

var (
	simpleMutex = sync.Mutex{}
	rwMutex     = sync.RWMutex{}
	waitGroup   = sync.WaitGroup{}
)

func consumerHandler(w http.ResponseWriter, r *http.Request) {
	var opts = []func(){
		func() {
			simpleMutex.Lock()
		},
		func() {
			simpleMutex.Unlock()
		},
		func() {
			rwMutex.RLock()
		},
		func() {
			rwMutex.RUnlock()
		},
		func() {
			rwMutex.Lock()
		},
		func() {
			rwMutex.Unlock()
		},
		func() {
			waitGroup.Add(1)
		},
		func() {
			waitGroup.Done()
		},
		func() {
			waitGroup.Wait()
		},
	}

	// for the validation of span order(span channel notify mechanism)
	for _, o := range opts {
		o()
		time.Sleep(time.Millisecond * 5)
	}

	w.Write([]byte("ok"))
}

func main() {
	http.HandleFunc("/consumer", consumerHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}
