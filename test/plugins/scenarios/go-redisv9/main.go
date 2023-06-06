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
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"net/http"

	_ "github.com/apache/skywalking-go"
)

var rdb *redis.Client

type testFunc func(ctx context.Context) error

type User struct {
	ID   uint
	Name string
	Age  uint8
}

func executeHandler(w http.ResponseWriter, r *http.Request) {
	testCases := []struct {
		name string
		fn   testFunc
	}{
		{"set_and_get", TestSetAndGet},
		{"pipeline_set_and_get", TestPipelineSetAndGet},
	}

	for _, test := range testCases {
		log.Printf("excute test case %s", test.name)
		if err := test.fn(r.Context()); err != nil {
			log.Fatalf("test case %s failed: %v", test.name, err)
		}
	}
	_, _ = w.Write([]byte("execute redis op success"))
}

func TestSetAndGet(ctx context.Context) error {
	key := "key_TestSetAndGet"
	value := "value_TestSetAndGet"
	if _, err := rdb.Set(ctx, key, value, 0).Result(); err != nil {
		return fmt.Errorf("SET error: %s", err.Error())
	}

	if v, err := rdb.Get(ctx, key).Result(); err != nil || v != value {
		return fmt.Errorf("GET error: %s", err.Error())
	}

	return nil
}

func TestPipelineSetAndGet(ctx context.Context) error {
	key1 := "key_TestPipelineSetAndGet_1"
	value1 := "value_TestPipelineSetAndGet_1"

	key2 := "key_TestPipelineSetAndGet_2"
	value2 := "value_TestPipelineSetAndGet_2"

	pipe := rdb.Pipeline()

	pipe.Set(ctx, key1, value1, 0)
	pipe.Set(ctx, key2, value2, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("pipeline SET error: %s", err.Error())
	}

	pipe = rdb.Pipeline()

	pipe.Get(ctx, key1)
	pipe.Get(ctx, key2)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("pipeline SET error: %s", err.Error())
	}

	for _, cmd := range cmds {
		if _, ok := cmd.(*redis.StringCmd); !ok {
			return fmt.Errorf("pipeline GET response not StringCmd type")
		}
	}
	return nil
}

func main() {
	c := redis.NewClient(&redis.Options{
		Addr:     "redis-server:6379",
		Password: "",
	})

	_, err := c.Ping(context.TODO()).Result()
	if err != nil {
		log.Fatalf("connect to redis error: %v \n", err)
	}

	rdb = c

	http.HandleFunc("/execute", executeHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}
