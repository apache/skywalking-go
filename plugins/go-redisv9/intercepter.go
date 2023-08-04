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
	"fmt"
	"github.com/apache/skywalking-go/plugins/core/operator"
	redis "github.com/redis/go-redis/v9"
)

type GoRedisInterceptor struct {
}

// BeforeInvoke would be called before the target method invocation.
func (g *GoRedisInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	return nil
}

// AfterInvoke would be called after the target method invocation.
func (g *GoRedisInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	rdb, ok := result[0].(redis.UniversalClient)
	if !ok {
		return fmt.Errorf("go-redis :skyWalking cannot create hook for client not match UniversalClient: %T", rdb)
	}
	switch c := rdb.(type) {
	case *redis.Client:
		c.AddHook(newRedisHook(c.Options().Addr))
	case *redis.ClusterClient:
		c.OnNewNode(func(rdb *redis.Client) {
			rdb.AddHook(newRedisHook(rdb.String()))
		})
	case *redis.Ring:
		c.OnNewNode(func(rdb *redis.Client) {
			rdb.AddHook(newRedisHook(rdb.String()))
		})
	default:
		return fmt.Errorf("go-redis :skyWalking cannot create hook for the unsupported client type: %T", c)
	}

	return nil
}
