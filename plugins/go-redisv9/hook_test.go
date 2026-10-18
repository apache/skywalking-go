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

package goredisv9

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestIsConnectionHandshake(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		cmd  redis.Cmder
		want bool
	}{
		{"client setinfo", redis.NewStatusCmd(ctx, "client", "setinfo", "LIB-NAME", "go-redis"), true},
		{"client maint_notifications", redis.NewStatusCmd(ctx, "client", "maint_notifications", "on"), true},
		{"upper case", redis.NewStatusCmd(ctx, "CLIENT", "SETINFO", "LIB-VER", "9.23.0"), true},
		{"hello with credentials", redis.NewMapStringInterfaceCmd(ctx, "hello", 3, "auth", "user", "secret"), true},
		{"auth", redis.NewStatusCmd(ctx, "auth", "secret"), true},
		{"select", redis.NewStatusCmd(ctx, "select", 1), true},
		{"readonly", redis.NewStatusCmd(ctx, "readonly"), true},
		{"client setname", redis.NewBoolCmd(ctx, "client", "setname", "app"), true},
		{"client tracking", redis.NewStatusCmd(ctx, "client", "tracking", "on"), true},
		{"client list", redis.NewStringCmd(ctx, "client", "list"), false},
		{"client without subcommand", redis.NewStatusCmd(ctx, "client"), false},
		{"get", redis.NewStringCmd(ctx, "get", "setinfo"), false},
	}
	for _, tt := range tests {
		if got := isConnectionHandshake(tt.cmd); got != tt.want {
			t.Errorf("%s: isConnectionHandshake() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsConnectionHandshakePipeline(t *testing.T) {
	ctx := context.Background()
	setinfo := redis.NewStatusCmd(ctx, "client", "setinfo", "LIB-NAME", "go-redis")
	get := redis.NewStringCmd(ctx, "get", "key")
	if !isConnectionHandshakePipeline([]redis.Cmder{setinfo, setinfo}) {
		t.Error("a pipeline of handshake commands should be skipped")
	}
	if isConnectionHandshakePipeline([]redis.Cmder{setinfo, get}) {
		t.Error("a pipeline with a user command must be traced")
	}
	if isConnectionHandshakePipeline(nil) {
		t.Error("an empty pipeline is not a handshake")
	}
}
