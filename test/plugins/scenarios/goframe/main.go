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

//go:nolint
import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/apache/skywalking-go"
)

//go:nolint
func main() {
	command := gcmd.Command{
		Name:  "goframe",
		Usage: "goframe",
		Brief: "start http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server()
			s.SetPort(8080)
			s.BindHandler("/provider", func(r *ghttp.Request) {
				r.Response.Write("success")
			})
			s.BindHandler("/consumer", func(r *ghttp.Request) {
				client := g.Client()
				client.SetHeader("h1", "h1-value")
				client.SetHeader("h2", "h2-value")
				var resp, err = client.Get(gctx.GetInitCtx(), "http://localhost:8080/provider?test=1")
				if err != nil {
					r.Response.Write(err.Error())
					return
				}
				defer resp.Close()
				var str = resp.ReadAllString()
				r.Response.Write(str)

				if err := requestStream("http://localhost:8080/stream"); err != nil {
					log.Printf("request stream error: %v", err)
				}
				if err := requestWebSocketUpgrade("http://localhost:8080/ws"); err != nil {
					log.Printf("request websocket error: %v", err)
				}
			})
			// streams an event with Response.Flush, which flushes through the
			// http.Flusher of the server's writer
			s.BindHandler("/stream", func(r *ghttp.Request) {
				r.Response.Header().Set("Content-Type", "text/event-stream")
				r.Response.Write("data: ping\n\n")
				r.Response.Flush()
				select {
				case <-streamReceived:
					r.Response.Write("data: done\n\n")
				case <-time.After(2 * time.Second):
					r.Response.Write("data: timeout\n\n")
				}
			})
			// hijacks the connection through the http.Hijacker of the server's writer
			s.BindHandler("/ws", func(r *ghttp.Request) {
				ws, err := r.WebSocket()
				if err != nil {
					r.Response.WriteStatus(http.StatusInternalServerError, err.Error())
					return
				}
				_ = ws.Close()
			})
			s.BindHandler("/health", func(r *ghttp.Request) {
				r.Response.WriteHeader(http.StatusOK)
				r.Response.Write("success")
			})
			s.Run()
			return nil
		},
	}
	command.Run(gctx.New())

}

// streamReceived tells the /stream handler that the client has read the first event.
var streamReceived = make(chan struct{}, 1)

// requestStream reads the event stream of /stream, which holds the response open
// until the first event was read. The response headers only arrive in time if
// the first event was flushed, otherwise the request times out.
func requestStream(url string) error {
	client := &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: time.Second}}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	first, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	select {
	case streamReceived <- struct{}{}:
	default:
	}
	rest, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK || first != "data: ping\n" || string(rest) != "\ndata: done\n\n" {
		return fmt.Errorf("unexpected stream: %d %q %q", resp.StatusCode, first, rest)
	}
	return nil
}

// requestWebSocketUpgrade sends a WebSocket handshake and expects 101 Switching Protocols.
func requestWebSocketUpgrade(url string) error {
	request, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("unexpected websocket upgrade status: %d", resp.StatusCode)
	}
	return nil
}
