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
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	_ "github.com/apache/skywalking-go"
	"github.com/apache/skywalking-go/toolkit/trace"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	http.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to /info", r.Method)

		sleepTime := rand.Intn(500) + 500
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)

		_, _ = w.Write([]byte("This is provider info"))
		log.Printf("Provider processed %s request to /info", r.Method)
	})

	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to /users", r.Method)
		sleepTime := rand.Intn(500) + 500
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		user := User{
			ID:   1,
			Name: "skywalking-go",
		}
		jsonData, err := json.Marshal(user)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error encoding JSON: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jsonData)
		log.Printf("Provider processed %s request to /users", r.Method)
	})

	http.HandleFunc("/correlation", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to /correlation", r.Method)
		sleepTime := rand.Intn(500) + 500
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		trace.SetCorrelation("PROVIDER_KEY", "provider")

		correlation := trace.GetCorrelation("PROVIDER_KEY") + "_" + trace.GetCorrelation("MIDDLE_KEY") + "_" + trace.GetCorrelation("PROVIDER_KEY")
		_, _ = w.Write([]byte(correlation))
		log.Printf("Provider processed %s request to /correlation", r.Method)
	})

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
