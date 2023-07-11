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

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLogContextStringGenerator struct {
}

func (z *ZapLogContextStringGenerator) String() string {
	return GetLogContextString()
}

func UpdateZapLogger(l *zap.Logger) {
	ChangeLogger(NewZapAdapter(l))
}

func TracingContextEnhance(entry *zapcore.CheckedEntry) (interface{}, *zapcore.Field) {
	var getEndpoint = LogReporterEnable
	if LogReporterEnable || LogTracingContextEnable {
		ctx := GetLogContext(getEndpoint)
		f := zap.String(LogTracingContextKey, ctx.String())
		return ctx, &f
	}
	return nil, nil
}

func KnownFieldFilter(fs, existingFields []zapcore.Field) []zapcore.Field {
	keys := LogReporterLabelKeys
	res := make([]zapcore.Field, 0)
	for _, k := range keys {
		for _, f := range fs {
			if f.Key == k {
				res = append(res, f)
			}
		}
	}
	// copy original data
	res = append(res, existingFields...)
	return res
}

type ZapAdapter struct {
	log *zap.Logger
}

func NewZapAdapter(log *zap.Logger) *ZapAdapter {
	return &ZapAdapter{log: log}
}

func (l *ZapAdapter) WithField(key string, value interface{}) interface{} {
	return NewZapAdapter(l.log.With(zap.Any(key, value)))
}

func (l *ZapAdapter) Info(args ...interface{}) {
	l.log.Info(fmt.Sprint(args...))
}

func (l *ZapAdapter) Infof(format string, args ...interface{}) {
	l.log.Info(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Warn(args ...interface{}) {
	l.log.Warn(fmt.Sprint(args...))
}

func (l *ZapAdapter) Warnf(format string, args ...interface{}) {
	l.log.Warn(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Error(args ...interface{}) {
	l.log.Error(fmt.Sprint(args...))
}

func (l *ZapAdapter) Errorf(format string, args ...interface{}) {
	l.log.Error(fmt.Sprintf(format, args...))
}
