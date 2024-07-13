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
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	_ "github.com/apache/skywalking-go"

	"github.com/gorilla/mux"
)

func provider(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Millisecond * 10)
	// test ws
	connectWs()
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

var upgrader = websocket.Upgrader{}

// ws
// ISSUE: https://github.com/apache/skywalking-go/pull/188
// Test http.ResponseWriter cast to http.Hijacker
func ws(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func connectWs() {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()
	c.WriteMessage(websocket.TextMessage, []byte("hello from mux test"))
	c.Close()
}

func main() {
	r := mux.NewRouter()
	r.Path("/health").HandlerFunc(health)
	r.Path("/consumer").HandlerFunc(consumer)
	r.PathPrefix("/provider").Path("/{var}").HandlerFunc(provider)
	r.Path("/ws").HandlerFunc(ws)

	// Bind to a port and pass our router in
	log.Fatal(http.ListenAndServe(":8080", r))
}
