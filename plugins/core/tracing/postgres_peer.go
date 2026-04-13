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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package tracing

type PostgreSQLAddress struct {
	Host string
	Port uint16
}

func BuildPostgreSQLPeer(primary PostgreSQLAddress, fallbacks []PostgreSQLAddress) string {
	addresses := make([]string, 0, len(fallbacks)+1)
	addresses = appendPostgreSQLPeerAddress(addresses, primary)
	for _, fallback := range fallbacks {
		addresses = appendPostgreSQLPeerAddress(addresses, fallback)
	}
	return joinStrings(addresses, ",")
}

func appendPostgreSQLPeerAddress(addresses []string, address PostgreSQLAddress) []string {
	if address.Host == "" {
		return addresses
	}
	port := uint16ToString(address.Port)
	formatted := address.Host + ":" + port
	if hasPrefix(address.Host, "/") {
		if hasSuffix(address.Host, "/") {
			formatted = address.Host + ".s.PGSQL." + port
		} else {
			formatted = address.Host + "/.s.PGSQL." + port
		}
	} else if countRune(address.Host, ':') > 1 && !hasPrefix(address.Host, "[") {
		formatted = "[" + address.Host + "]:" + port
	}
	for _, existed := range addresses {
		if existed == formatted {
			return addresses
		}
	}
	return append(addresses, formatted)
}

func joinStrings(items []string, separator string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	size := 0
	for _, item := range items {
		size += len(item)
	}
	size += len(separator) * (len(items) - 1)
	buf := make([]byte, 0, size)
	for i, item := range items {
		if i > 0 {
			buf = append(buf, separator...)
		}
		buf = append(buf, item...)
	}
	return string(buf)
}

func hasPrefix(val, prefix string) bool {
	if len(prefix) > len(val) {
		return false
	}
	return val[:len(prefix)] == prefix
}

func hasSuffix(val, suffix string) bool {
	if len(suffix) > len(val) {
		return false
	}
	return val[len(val)-len(suffix):] == suffix
}

func countRune(val string, target byte) int {
	count := 0
	for i := 0; i < len(val); i++ {
		if val[i] == target {
			count++
		}
	}
	return count
}

func uint16ToString(val uint16) string {
	if val == 0 {
		return "0"
	}
	var buf [5]byte
	pos := len(buf)
	for val > 0 {
		pos--
		buf[pos] = byte('0' + val%10)
		val /= 10
	}
	return string(buf[pos:])
}
