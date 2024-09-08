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

package log

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
)

const (
	debugLevel = "debug"
	infoLevel  = "info"
	warnLevel  = "warn"
	errorLevel = "error"
)

func sendLogEntry(level string, args ...interface{}) {
	if len(args) == 0 {
		return
	}
	logReporter := operator.GetOperator().LogReporter().(operator.LogReporter)
	msg := args[0].(string)
	labels := parseLabels(args[1])
	logReporter.ReportLog(logReporter.GetLogContext(true), args[1], level, msg, labels)
}

// parseLabels parses multiple args into a map of labels
func parseLabels(args interface{}) map[string]string {
	keyValues, ok := args.([]string)
	if !ok || len(keyValues) < 2 {
		return nil
	}

	ret := make(map[string]string)
	for i := 0; i < len(keyValues); i += 2 {
		v1 := keyValues[i]
		v2 := keyValues[i+1]
		ret[v1] = v2
	}

	return ret
}
