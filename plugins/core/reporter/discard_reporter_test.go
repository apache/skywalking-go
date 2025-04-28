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

package reporter_test

import (
	"testing"

	"github.com/apache/skywalking-go/plugins/core/reporter"
)

func TestDiscardReporter(t *testing.T) {
	r := reporter.NewDiscardReporter()

	// make sure no panic
	r.Boot(nil, nil)
	r.SendTracing(nil)
	r.SendMetrics(nil)
	r.SendLog(nil)
	if status := r.ConnectionStatus(); status != reporter.ConnectionStatusDisconnect {
		t.Errorf("expect 2, actual is %d", status)
	}
	r.Close()
}
