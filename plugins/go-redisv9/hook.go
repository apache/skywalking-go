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
	"fmt"
	"net"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/tracing"

	"github.com/redis/go-redis/v9"
)

const (
	GoRedisComponentID = 5014
	GoRedisCacheType   = "redis"
)

type redisHook struct {
	Addr string
}

func newRedisHook(addr string) *redisHook {
	return &redisHook{
		Addr: addr,
	}
}

func (r *redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		s, err := tracing.CreateExitSpan(
			// operationName
			GoRedisCacheType+"/"+"dial",

			// peer
			r.Addr,

			// injector
			func(k, v string) error {
				return nil
			},

			// opts
			tracing.WithComponent(GoRedisComponentID),
			tracing.WithLayer(tracing.SpanLayerCache),
			tracing.WithTag(tracing.TagCacheType, GoRedisCacheType),
		)

		if err != nil {
			err = fmt.Errorf("go-redis :skyWalking failed to create exit span, got error: %v", err)
			return nil, err
		}

		defer s.End()

		conn, err := next(ctx, network, addr)
		if err != nil {
			recordError(s, err)
		}
		return conn, err
	}
}

func (r *redisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if isConnectionHandshake(cmd) {
			return next(ctx, cmd)
		}
		var s tracing.Span
		var err error
		if r.Addr != "" {
			s, err = tracing.CreateExitSpan(
				// operationName
				GoRedisCacheType+"/"+cmd.FullName(),

				// peer
				r.Addr,

				// injector
				func(k, v string) error {
					return nil
				},

				// opts
				tracing.WithComponent(GoRedisComponentID),
				tracing.WithLayer(tracing.SpanLayerCache),
				tracing.WithTag(tracing.TagCacheType, GoRedisCacheType),
				tracing.WithTag(tracing.TagCacheOp, getCacheOp(cmd.FullName())),
				tracing.WithTag(tracing.TagCacheCmd, cmd.FullName()),
				tracing.WithTag(tracing.TagCacheKey, getKey(cmd.Args())),
				tracing.WithTag(tracing.TagCacheArgs, maxString(cmd.String(), config.MaxArgsBytes)),
			)

			if err != nil {
				err = fmt.Errorf("go-redis :skyWalking failed to create exit span, got error: %v", err)
				return err
			}

			defer s.End()
		}

		if err = next(ctx, cmd); err != nil {
			recordError(s, err)
			return err
		}

		return nil
	}
}

func (r *redisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if isConnectionHandshakePipeline(cmds) {
			return next(ctx, cmds)
		}
		summary := ""
		summaryCmds := cmds
		if len(summaryCmds) > 10 {
			summaryCmds = summaryCmds[:10]
		}
		for i := range summaryCmds {
			summary += summaryCmds[i].FullName() + "/"
		}
		if len(cmds) > 10 {
			summary += "..."
		}

		var s tracing.Span
		var err error
		if r.Addr != "" {
			s, err = tracing.CreateExitSpan(
				// operationName
				"redis/pipeline",

				// peer
				r.Addr,

				// injector
				func(k, v string) error {
					return nil
				},

				// opts
				tracing.WithComponent(GoRedisComponentID),
				tracing.WithLayer(tracing.SpanLayerCache),
				tracing.WithTag(tracing.TagCacheType, GoRedisCacheType),
				tracing.WithTag(tracing.TagCacheCmd, "pipeline:"+strings.TrimRight(summary, "/")),
			)
			if err != nil {
				err = fmt.Errorf("go-redis :skyWalking failed to create exit span, got error: %v", err)
				return err
			}
			defer s.End()
		}

		if err = next(ctx, cmds); err != nil {
			recordError(s, err)
			return err
		}

		return nil
	}
}

// isConnectionHandshake reports whether the command is one that go-redis sends on its
// own while initializing a new connection. Since v9.9.0 the connection setup runs
// through the client hooks: HELLO (with credentials), the AUTH fallback, SELECT,
// READONLY, and CLIENT SETNAME, SETINFO, MAINT_NOTIFICATIONS, and TRACKING (client-side
// caching). They are not user
// operations, and HELLO and AUTH carry the password, so no span is created for them.
func isConnectionHandshake(cmd redis.Cmder) bool {
	switch strings.ToLower(cmd.Name()) {
	case "hello", "auth", "select", "readonly":
		return true
	case "client":
	default:
		return false
	}
	args := cmd.Args()
	if len(args) < 2 {
		return false
	}
	sub, ok := args[1].(string)
	if !ok {
		return false
	}
	switch strings.ToLower(sub) {
	case "setname", "setinfo", "maint_notifications", "tracking":
		return true
	}
	return false
}

func isConnectionHandshakePipeline(cmds []redis.Cmder) bool {
	if len(cmds) == 0 {
		return false
	}
	for _, cmd := range cmds {
		if !isConnectionHandshake(cmd) {
			return false
		}
	}
	return true
}

func recordError(span tracing.Span, err error) {
	if err != redis.Nil && span != nil {
		span.Error(err.Error())
	}
}

// getKey Try to transform the second argument into string
// e.g. "GET my_key" -> "my_key"
func getKey(args []interface{}) string {
	key := ""
	if len(args) >= 2 {
		k := args[1]
		switch v := k.(type) {
		case string:
			key = v
		default:
			break
		}
	}
	return key
}

// maxString limit the bytes length of the redis args.
func maxString(s string, length int) string {
	if length <= 0 { // no define or no limit
		return s
	}

	if len(s) > length {
		return s[:length]
	}
	return s
}
