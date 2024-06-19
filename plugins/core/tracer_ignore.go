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

package core

import (
	"strings"
)

func tracerIgnore(operationName string, ignoreSuffixList, ignorePath []string) bool {
	return ignoreSuffix(operationName, ignoreSuffixList) || traceIgnorePath(operationName, ignorePath)
}

func ignoreSuffix(operationName string, ignoreSuffix []string) bool {
	if len(ignoreSuffix) == 0 {
		return false
	}
	suffixIdx := strings.LastIndex(operationName, ".")
	if suffixIdx == -1 {
		return false
	}
	for _, suffix := range ignoreSuffix {
		if suffix == operationName[suffixIdx:] {
			return true
		}
	}
	return false
}

func traceIgnorePath(operationName string, ignorePath []string) bool {
	if len(ignorePath) == 0 {
		return false
	}
	for _, pattern := range ignorePath {
		if normalMatch(pattern, 0, operationName, 0) {
			return true
		}
	}
	return false
}

// normalMatch determines whether the operation name matches the wildcard pattern.
// The parameters `p` and `s` represent the current index in pattern and operationName respectively.
func normalMatch(pattern string, p int, operationName string, s int) bool {
	for p < len(pattern) {
		pc := pattern[p]
		sc := safeCharAt(operationName, s)

		if pc == '*' {
			p++
			if safeCharAt(pattern, p) == '*' {
				p++
				return multiWildcardMatch(pattern, p, operationName, s)
			}
			return wildcardMatch(pattern, p, operationName, s)
		}

		if (pc == '?' && sc != 0 && sc != '/') || pc == sc {
			s++
			p++
			continue
		}
		return false
	}
	return s == len(operationName)
}

func wildcardMatch(pattern string, p int, operationName string, s int) bool {
	pc := safeCharAt(pattern, p)

	if pc == 0 {
		for {
			sc := safeCharAt(operationName, s)
			if sc == 0 {
				return true
			}
			if sc == '/' {
				return s == len(operationName)-1
			}
			s++
		}
	}

	for {
		sc := safeCharAt(operationName, s)
		if sc == '/' {
			if pc == sc {
				return normalMatch(pattern, p+1, operationName, s+1)
			}
			return false
		}
		if !normalMatch(pattern, p, operationName, s) {
			if s >= len(operationName) {
				return false
			}
			s++
			continue
		}
		return true
	}
}

func multiWildcardMatch(pattern string, p int, operationName string, s int) bool {
	switch safeCharAt(pattern, p) {
	case 0:
		return true
	case '/':
		p++
	}
	for {
		if !normalMatch(pattern, p, operationName, s) {
			if s >= len(operationName) {
				return false
			}
			s++
			continue
		}
		return true
	}
}

func safeCharAt(value string, index int) byte {
	if index >= len(value) {
		return 0
	}
	return value[index]
}
