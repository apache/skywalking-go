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

const (
	opTypeWrite   = "write"
	opTypeRead    = "read"
	opTypeUnknown = ""
)

// Commands are divided into different type.
//  Ref: https://github.com/apache/skywalking-java/blob/main/apm-sniffer/apm-sdk-plugin/jedis-plugins/jedis-2.x-3.x-plugin/src/main/java/org/apache/skywalking/apm/plugin/jedis/v3/JedisPluginConfig.java

var writeOperation = map[string]bool{
	"getset":           true,
	"set":              true,
	"setbit":           true,
	"setex":            true,
	"setnx":            true,
	"setrange":         true,
	"strlen":           true,
	"mset":             true,
	"msetnx":           true,
	"psetex":           true,
	"incr":             true,
	"incrby":           true,
	"incrbyfloat":      true,
	"decr":             true,
	"decrby":           true,
	"append":           true,
	"hmset":            true,
	"hset":             true,
	"hsetnx":           true,
	"hincrby":          true,
	"hincrbyfloat":     true,
	"hdel":             true,
	"rpoplpush":        true,
	"rpush":            true,
	"rpushx":           true,
	"lpush":            true,
	"lpushx":           true,
	"lrem":             true,
	"ltrim":            true,
	"lset":             true,
	"brpoplpush":       true,
	"linsert":          true,
	"sadd":             true,
	"sdiff":            true,
	"sdiffstore":       true,
	"sinterstore":      true,
	"sismember":        true,
	"srem":             true,
	"sunion":           true,
	"sunionstore":      true,
	"sinter":           true,
	"zadd":             true,
	"zincrby":          true,
	"zinterstore":      true,
	"zrange":           true,
	"zrangebylex":      true,
	"zrangebyscore":    true,
	"zrank":            true,
	"zrem":             true,
	"zremrangebylex":   true,
	"zremrangebyrank":  true,
	"zremrangebyscore": true,
	"zrevrange":        true,
	"zrevrangebyscore": true,
	"zrevrank":         true,
	"zunionstore":      true,
	"xadd":             true,
	"xdel":             true,
	"del":              true,
	"xtrim":            true,
}

var readOperation = map[string]bool{
	"getrange":    true,
	"getbit":      true,
	"mget":        true,
	"hvals":       true,
	"hkeys":       true,
	"hlen":        true,
	"hexists":     true,
	"hget":        true,
	"hgetall":     true,
	"hmget":       true,
	"blpop":       true,
	"brpop":       true,
	"lindex":      true,
	"llen":        true,
	"lpop":        true,
	"lrange":      true,
	"rpop":        true,
	"scard":       true,
	"srandmember": true,
	"spop":        true,
	"sscan":       true,
	"smove":       true,
	"zlexcount":   true,
	"zscore":      true,
	"zscan":       true,
	"zcard":       true,
	"zcount":      true,
	"xget":        true,
	"get":         true,
	"xread":       true,
	"xlen":        true,
	"xrange":      true,
	"xrevrange":   true,
}

// getCacheOp return "read" or "write" or "" based on the cmd.
// https://github.com/apache/skywalking-java/blob/main/apm-sniffer/optional-plugins/ehcache-2.x-plugin/src/main/java/org/apache/skywalking/apm/plugin/ehcache/v2/EncacheOperationConvertor.java#L29
func getCacheOp(cmd string) string {
	if readOperation[cmd] {
		return opTypeRead
	} else if writeOperation[cmd] {
		return opTypeWrite
	}
	return opTypeUnknown
}
