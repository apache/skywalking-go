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

package frameworks

import (
	"fmt"

	"go.uber.org/zap/zapcore"
)

func ReportLogFromZapEntry(entry *zapcore.CheckedEntry, fields, needs []zapcore.Field, tracingContext interface{},
	tracingContextField *zapcore.Field, reporterEnable, logEnable bool, reportLabelsKeys []string) []zapcore.Field {
	if reporterEnable && tracingContext != nil {
		labels := make(map[string]string, len(reportLabelsKeys))
		for _, key := range reportLabelsKeys {
			for _, f := range fields {
				if f.Key == key {
					if k, v := generateLabelKeyValueFromField(f); k != "" {
						labels[k] = v
					}
				}
			}
		}
		for _, f := range needs {
			if k, v := generateLabelKeyValueFromField(f); k != "" {
				labels[k] = v
			}
		}
		ReportLog(tracingContext, entry.Time, entry.Level.String(), entry.Message, labels)
	}
	if logEnable && tracingContextField != nil {
		fields = append(fields, *tracingContextField)
	}
	return fields
}

func generateLabelKeyValueFromField(field zapcore.Field) (key, value string) {
	if field.Interface != nil {
		return field.Key, fmt.Sprintf("%v", field.Interface)
	} else if field.Integer > 0 {
		return field.Key, fmt.Sprintf("%d", field.Integer)
	} else if field.String != "" {
		return field.Key, field.String
	}
	return "", ""
}
