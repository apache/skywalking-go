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

package core

import (
	"time"

	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	logv3 "skywalking.apache.org/repo/goapi/collect/logging/v3"
)

func (t *Tracer) LogReporter() interface{} {
	return t
}

type logTracingContext interface {
	GetServiceName() string
	GetInstanceName() string
	GetTraceID() string
	GetTraceSegmentID() string
	GetSpanID() int32
	GetEndPointName() string
}

func (t *Tracer) ReportLog(ctx, timeObj interface{}, level, msg string, labels map[string]string) {
	tracingContext, ok := ctx.(logTracingContext)
	if !ok || tracingContext == nil {
		return
	}
	entity := t.ServiceEntity
	if entity == nil {
		return
	}
	timeData := timeObj.(time.Time)

	tags := &logv3.LogTags{
		Data: []*commonv3.KeyStringValuePair{
			{
				Key:   "LEVEL",
				Value: level,
			},
		},
	}
	for k, v := range labels {
		tags.Data = append(tags.Data, &commonv3.KeyStringValuePair{
			Key:   k,
			Value: v,
		})
	}

	logData := &logv3.LogData{
		Timestamp:       Millisecond(timeData),
		Service:         tracingContext.GetServiceName(),
		ServiceInstance: tracingContext.GetInstanceName(),
		Endpoint:        tracingContext.GetEndPointName(),
		Body: &logv3.LogDataBody{
			Type: "TEXT",
			Content: &logv3.LogDataBody_Text{
				Text: &logv3.TextLog{Text: msg},
			},
		},
		TraceContext: &logv3.TraceContext{
			TraceId:        tracingContext.GetTraceID(),
			TraceSegmentId: tracingContext.GetTraceSegmentID(),
			SpanId:         tracingContext.GetSpanID(),
		},
		Tags:  tags,
		Layer: "GENERAL",
	}

	t.Reporter.SendLog(logData)
}
