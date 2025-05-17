// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	_ "github.com/apache/skywalking-go"
	"github.com/apache/skywalking-go/toolkit/logging"
	"github.com/apache/skywalking-go/toolkit/trace"
)

var providerURL string

func main() {

	providerURL = os.Getenv("PROVIDER_URL")
	if providerURL == "" {
		providerURL = "http://localhost:8080"
	}
	log.Printf("Provider URL: %s\n", providerURL)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to /info", r.Method)

		sleepTime := rand.Intn(500) + 500
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)

		var resp *http.Response
		var err error

		resp, err = http.Post(providerURL+"/info", "application/json", bytes.NewBufferString(""))
		if err != nil {
			http.Error(w, fmt.Sprintf("Error calling provider: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading provider response: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)

		logging.Debug("this is debug msg", "foo1", "bar1")
		logging.Info("this is info msg", "foo2", "bar2")
		logging.Warn("this is warn msg", "foo3", "bar3")
		logging.Error("this is error msg", "foo4", "bar4")
		log.Printf("Consumer processed %s request to /info", r.Method)
	})

	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to /users", r.Method)

		sleepTime := rand.Intn(500) + 500
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)

		var resp *http.Response
		var err error

		resp, err = http.Post(providerURL+"/users", "application/json", bytes.NewBufferString("{}"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Error calling provider: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading provider response: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		log.Printf("Consumer processed %s request to /users", r.Method)
	})

	http.HandleFunc("/correlation", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to /correlation", r.Method)

		sleepTime := rand.Intn(500) + 500
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		trace.SetCorrelation("CONSUMER_KEY", "consumer")

		var resp *http.Response
		var err error

		resp, err = http.Post(providerURL+"/correlation", "application/json", bytes.NewBufferString(""))
		if err != nil {
			http.Error(w, fmt.Sprintf("Error calling provider: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading provider response: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		log.Printf("Consumer processed %s request to /correlation", r.Method)
	})

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
