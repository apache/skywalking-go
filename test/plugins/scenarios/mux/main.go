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

	"github.com/gorilla/mux"
)

func provider(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Millisecond * 10)
	w.Write([]byte("success"))
}

func consumer(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://localhost:8080/provider/test?test=1")
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
	_, _ = w.Write(body)
}

func health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("success"))
}

func main() {
	r := mux.NewRouter()
	r.Path("/health").HandlerFunc(health)
	r.Path("/consumer").HandlerFunc(consumer)
	r.PathPrefix("/provider").Path("/{var}").HandlerFunc(provider)

	// Bind to a port and pass our router in
	log.Fatal(http.ListenAndServe(":8080", r))
}
