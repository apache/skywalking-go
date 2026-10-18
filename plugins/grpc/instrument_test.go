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

package grpc

import "testing"

func TestVersionChecker(t *testing.T) {
	tests := map[string]bool{
		"v1.55.0":                               true,
		"v1.56.2":                               true,
		"v1.81.1":                               true,
		"v1.83.2":                               true,
		"v1.83.9":                               true,
		"v1.83.3-0.20260601000000-abcdefabcdef": true,
		"v1.84.0":                               false,
		"v1.84.0-dev":                           false,
		"v1.85.1":                               false,
		"v2.0.0":                                false,
		"":                                      true,
		"invalid":                               true,
	}
	inst := NewInstrument()
	for version, want := range tests {
		if got := inst.VersionChecker(version); got != want {
			t.Errorf("VersionChecker(%q) = %v, want %v", version, got, want)
		}
	}
}
