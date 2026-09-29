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

package reporter

import (
	"testing"
	"time"
)

func TestKeepaliveParamsTimeFromCheckInterval(t *testing.T) {
	cases := []struct {
		name          string
		checkInterval time.Duration
		wantTime      time.Duration
	}{
		{name: "default floor", checkInterval: 20 * time.Second, wantTime: 30 * time.Second},
		{name: "zero treated as floor", checkInterval: 0, wantTime: 30 * time.Second},
		{name: "negative treated as floor", checkInterval: -time.Second, wantTime: 30 * time.Second},
		{name: "long interval", checkInterval: 2 * time.Minute, wantTime: 2*time.Minute + 10*time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cm := &ConnectionManager{checkInterval: tc.checkInterval}
			got := cm.keepaliveParams()
			if got.Time != tc.wantTime {
				t.Fatalf("Time = %v, want %v", got.Time, tc.wantTime)
			}
			if got.Timeout != 10*time.Second {
				t.Fatalf("Timeout = %v, want 10s", got.Timeout)
			}
			if !got.PermitWithoutStream {
				t.Fatal("PermitWithoutStream should be true")
			}
		})
	}
}
