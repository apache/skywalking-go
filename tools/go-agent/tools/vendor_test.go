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
	"fmt"
	"testing"
)

func TestUnVendor(t *testing.T) {
	tests := []struct {
		pkgPath  string
		excepted string
	}{
		{
			pkgPath:  "github.com/apache/skywalking-go-plugins",
			excepted: "github.com/apache/skywalking-go-plugins",
		},
		{
			pkgPath:  "test/vendor",
			excepted: "test/vendor",
		},
		{
			pkgPath:  "application-path/vendor/github.com/apache/skywalking-go-plugins",
			excepted: "github.com/apache/skywalking-go-plugins",
		},
	}

	for _, test := range tests {
		actual := UnVendor(test.pkgPath)
		if actual != test.excepted {
			t.Errorf("UnVendor(%s) = %s, excepted %s", test.pkgPath, actual, test.excepted)
		}
	}
}

func TestParseVendorModule(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		hasError     bool
		packageCount int
		validate     func(modules VendorModules) error
	}{
		{
			path:         "./testdata/no-modules.txt",
			hasError:     false,
			packageCount: 0,
		},
		{
			path:         "./testdata/single-modules.txt",
			hasError:     false,
			packageCount: 2,
			validate: func(modules VendorModules) error {
				if m := modules["github.com/apache/skywalking-go/log"]; m == nil {
					return fmt.Errorf("module missing")
				} else if m.Version != "v0.3.0" {
					return fmt.Errorf("version not correct")
				}
				return nil
			},
		},
		{
			path:     "./testdata/wrong-modules.txt",
			hasError: true,
		},
		{
			path:         "./testdata/multi-modules.txt",
			hasError:     false,
			packageCount: 3,
			validate: func(modules VendorModules) error {
				if m := modules["github.com/bytedance/sonic"]; m == nil {
					return fmt.Errorf("module missing")
				} else if m.Version != "v1.9.1" {
					return fmt.Errorf("version not correct")
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		module, err := ParseVendorModule(test.path)
		if hasError := err != nil; test.hasError != hasError {
			t.Errorf("test %s has error, excepted: %t, got: %t", test.name, test.hasError, hasError)
		}
		if moduleCount := len(module); test.packageCount != moduleCount {
			t.Errorf("test %s module count, excepted: %d, got: %d", test.name, test.packageCount, moduleCount)
		}
		if test.validate != nil {
			if err := test.validate(module); err != nil {
				t.Errorf("test %s validate failure: %s", test.name, err)
			}
		}
	}
}
