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

package api

import (
	"path/filepath"
	"strconv"
	"strings"
)

type CompileOptions struct {
	Package string   `swflag:"-p"`
	Output  string   `swflag:"-o"`
	AllArgs []string `swflag:"all-args"`
	Lang    string   `swflag:"-lang"`

	DebugDir string `swflag:"-debug"` // from tools flag
}

func (c *CompileOptions) IsValid() bool {
	return c.Package != "" && c.Output != ""
}

func (c *CompileOptions) CompileBaseDir() string {
	return filepath.Dir(filepath.Dir(c.Output))
}

func (c *CompileOptions) CheckGoVersionGreaterOrEqual(requiredMajor, requiredMinor int) bool {
	if c.Lang == "" {
		return false
	}
	if !strings.HasPrefix(c.Lang, "go") {
		return false
	}
	versionStr := strings.TrimPrefix(c.Lang, "go")
	parts := strings.SplitN(versionStr, ".", 3)
	if len(parts) < 2 {
		return false
	}

	majorStr := parts[0]
	currentMajor64, err := strconv.ParseInt(majorStr, 10, 64)
	if err != nil {
		return false
	}
	currentMajor := int(currentMajor64)

	minorStr := parts[1]
	currentMinor64, err := strconv.ParseInt(minorStr, 10, 64)
	if err != nil {
		return false
	}
	currentMinor := int(currentMinor64)

	if currentMajor > requiredMajor {
		return true
	}
	if currentMajor == requiredMajor && currentMinor >= requiredMinor {
		return true
	}
	return false
}
