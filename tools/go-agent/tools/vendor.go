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

package tools

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

const VendorDir = "/vendor/"

type VendorModule struct {
	Name    string
	Version string
}

type VendorModules map[string]*VendorModule

// UnVendor removes the vendor directory from the path.
func UnVendor(path string) string {
	i := strings.Index(path, VendorDir)
	if i == -1 {
		return path
	}
	return path[i+len(VendorDir):]
}

func ParseVendorModule(path string) (VendorModules, error) {
	moduleContent, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	modules := make(VendorModules)
	var module *VendorModule
	for _, moduleData := range bytes.Split(moduleContent, []byte("\n")) {
		moduleString := strings.TrimSpace(string(moduleData))
		if strings.HasPrefix(moduleString, "# ") {
			// module
			moduleInfo := strings.SplitAfterN(moduleString, " ", 3)
			if len(moduleInfo) != 3 {
				return nil, fmt.Errorf("module data cannot be analyzed")
			}
			module = &VendorModule{
				Name:    moduleInfo[1],
				Version: moduleInfo[2],
			}
			continue
		} else if strings.HasPrefix(moduleString, "#") {
			// go version required, ignore
			continue
		} else if len(moduleString) == 0 {
			// empty data, ignore
			continue
		}

		// otherwise, it should be the module package path
		if module == nil {
			return nil, fmt.Errorf("cannot found previous module data")
		}
		modules[moduleString] = module
	}
	return modules, nil
}
