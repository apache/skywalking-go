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
	"time"

	_ "github.com/apache/skywalking-go"
)

func main() {
	http.HandleFunc("/so11y", so11yHandler)
	http.HandleFunc("/propagated", propagatedHandler)
	http.HandleFunc("/ignored.html", ignoredHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}

func so11yHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://localhost:8080/propagated")
	if err != nil {
		log.Printf("request propagated error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err = http.Get("http://localhost:8080/ignored.html")
	if err != nil {
		log.Printf("request ignored.html error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
	time.Sleep(2 * time.Second) // make sure the meter already uploaded
}

func propagatedHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(2 * time.Second) // make sure the meter already uploaded
	_, _ = w.Write([]byte("success"))
}

func ignoredHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(2 * time.Second) // make sure the meter already uploaded
	_, _ = w.Write([]byte("Nobody cares me."))
}
