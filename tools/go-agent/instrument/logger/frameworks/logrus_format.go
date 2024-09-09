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

	"github.com/sirupsen/logrus"
)

// WrapFormat is wrap format to transmit trace context when logging
type WrapFormat struct {
	Base            logrus.Formatter
	traceContextKey string
}

// Wrap original format
// nolint
func Wrap(base logrus.Formatter, contextKey string) *WrapFormat {
	if contextKey == "" {
		contextKey = "SW_CTX"
	}

	return &WrapFormat{base, contextKey}
}

// Format logging with trace context
func (format *WrapFormat) Format(entry *logrus.Entry) ([]byte, error) {
	var logContext fmt.Stringer
	keys := LogReporterLabelKeys
	if LogReporterEnable {
		ctx := GetLogContext(true)
		if ctx == nil {
			return format.Base.Format(entry)
		}
		if stringer, ok := ctx.(fmt.Stringer); ok {
			logContext = stringer
		}
		labels := make(map[string]string, len(keys))
		for _, key := range keys {
			for k, v := range entry.Data {
				if k == key {
					labels[key] = fmt.Sprintf("%v", v)
				}
			}
		}
		ReportLog(ctx, entry.Time, entry.Level.String(), entry.Message, labels)
	}
	// append trace context
	if logContext == nil {
		entry.Data[format.traceContextKey] = GetLogContextString()
	} else {
		entry.Data[format.traceContextKey] = logContext.String()
	}

	return format.Base.Format(entry)
}
