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
	"github.com/sirupsen/logrus"
)

func UpdateLogrusLogger(l *logrus.Logger) {
	// 添加保护性检查，防止在Go 1.24中由于init顺序变更导致的nil panic
	if l == nil {
		return
	}

	if LogTracingContextEnable {
		if _, wrapperd := l.Formatter.(*WrapFormat); !wrapperd {
			l.Formatter = Wrap(l.Formatter, LogTracingContextKey)
		}
	}

	// 确保ChangeLogger不是nil
	if ChangeLogger != nil {
		ChangeLogger(NewLogrusAdapter(l))
	}
}

type LogrusAdapter struct {
	log *logrus.Entry
}

func NewLogrusAdapter(log *logrus.Logger) *LogrusAdapter {
	return &LogrusAdapter{log: log.WithFields(logrus.Fields{})}
}

func (l *LogrusAdapter) WithField(key string, value interface{}) interface{} {
	return &LogrusAdapter{log: l.log.WithFields(logrus.Fields{key: value})}
}

func (l *LogrusAdapter) Info(args ...interface{}) {
	l.log.Info(args...)
}

func (l *LogrusAdapter) Infof(format string, args ...interface{}) {
	l.log.Infof(format, args...)
}

func (l *LogrusAdapter) Warn(args ...interface{}) {
	l.log.Warn(args...)
}

func (l *LogrusAdapter) Warnf(format string, args ...interface{}) {
	l.log.Warnf(format, args...)
}

func (l *LogrusAdapter) Error(args ...interface{}) {
	l.log.Error(args...)
}

func (l *LogrusAdapter) Errorf(format string, args ...interface{}) {
	l.log.Errorf(format, args...)
}
