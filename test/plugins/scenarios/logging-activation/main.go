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
	"net/http"

	_ "github.com/apache/skywalking-go"
	"github.com/apache/skywalking-go/toolkit/logging"
)

func providerHandler(w http.ResponseWriter, r *http.Request) {
	logging.Debug("this is debug msg", "foo1", "bar1")
	_, _ = w.Write([]byte("success"))
}

func consumerHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://localhost:8080/provider?test=1")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logging.Info("this is info msg", "foo2", "bar2")
	logging.Warn("this is warn msg", "foo3", "bar3")
	logging.Error("this is error msg", "foo4", "bar4")
	_, _ = w.Write(body)
}

func main() {
	http.HandleFunc("/provider", providerHandler)
	http.HandleFunc("/consumer", consumerHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}
