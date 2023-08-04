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
	"github.com/apache/skywalking-go/plugins/core"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func init() {
	core.ResetTracingContext()
}

func TestRedisClientInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &GoRedisInterceptor{}
	clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:          []string{"localhost:6379"},
		DialTimeout:    time.Second * 10,
		PoolSize:       10,
		RouteByLatency: true,
	})
	err := interceptor.AfterInvoke(nil, clusterClient)
	assert.Nil(t, err, "failed to invoke AfterInvoke")
	status, err := clusterClient.Ping(context.Background()).Result()
	assert.Nil(t, err, "ping err")
	assert.Equal(t, "PONG", status, "should be PONG")
}

func TestRedisClusterClientInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &GoRedisInterceptor{}
	clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:          []string{"localhost:6479", "localhost:6579"},
		DialTimeout:    time.Second * 10,
		PoolSize:       10,
		RouteByLatency: true,
	})
	err := interceptor.AfterInvoke(nil, clusterClient)
	assert.Nil(t, err, "failed to invoke AfterInvoke")
	status, err := clusterClient.Ping(context.Background()).Result()
	assert.Nil(t, err, "ping err")
	assert.Equal(t, "PONG", status, "should be PONG")
}

func TestRedisRingClientInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &GoRedisInterceptor{}
	clusterClient := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"shard1": "localhost:7000",
			"shard2": "localhost:7001",
		},
		DialTimeout: time.Second * 10,
		PoolSize:    10,
	})
	err := interceptor.AfterInvoke(nil, clusterClient)
	assert.Nil(t, err, "failed to invoke AfterInvoke")
	status, err := clusterClient.Ping(context.Background()).Result()
	assert.Nil(t, err, "ping err")
	assert.Equal(t, "PONG", status, "should be PONG")
}
