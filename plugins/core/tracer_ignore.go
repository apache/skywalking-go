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
	for _, pattern := range ignorePath {
		if normalMatch(pattern, 0, operationName, 0) {
			return true
		}
	}
	return false
}

func normalMatch(pat string, p int, str string, s int) bool {
	for p < len(pat) {
		pc := pat[p]
		sc := safeCharAt(str, s)

		if pc == '*' {
			p++
			if safeCharAt(pat, p) == '*' {
				p++
				return multiWildcardMatch(pat, p, str, s)
			}
			return wildcardMatch(pat, p, str, s)
		}

		if (pc == '?' && sc != 0 && sc != '/') || pc == sc {
			s++
			p++
			continue
		}
		return false
	}
	return s == len(str)
}

func wildcardMatch(pat string, p int, str string, s int) bool {
	pc := safeCharAt(pat, p)

	if pc == 0 {
		for {
			sc := safeCharAt(str, s)
			if sc == 0 {
				return true
			}
			if sc == '/' {
				return s == len(str)-1
			}
			s++
		}
	}

	for {
		sc := safeCharAt(str, s)
		if sc == '/' {
			if pc == sc {
				return normalMatch(pat, p+1, str, s+1)
			}
			return false
		}
		if !normalMatch(pat, p, str, s) {
			if s >= len(str) {
				return false
			}
			s++
			continue
		}
		return true
	}
}

func multiWildcardMatch(pat string, p int, str string, s int) bool {
	switch safeCharAt(pat, p) {
	case 0:
		return true
	case '/':
		p++
	}
	for {
		if !normalMatch(pat, p, str, s) {
			if s >= len(str) {
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
