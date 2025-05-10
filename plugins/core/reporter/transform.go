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
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	"time"
)

type Transform struct {
	entity *Entity
}

func NewTransform(entity *Entity) *Transform {
	return &Transform{
		entity: entity,
	}
}

func (r *Transform) TransformSegmentObject(spans []ReportedSpan) *agentv3.SegmentObject {
	spanSize := len(spans)
	if spanSize < 1 {
		return nil
	}
	rootSpan := spans[spanSize-1]
	rootCtx := rootSpan.Context()
	segmentObject := &agentv3.SegmentObject{
		TraceId:         rootCtx.GetTraceID(),
		TraceSegmentId:  rootCtx.GetSegmentID(),
		Spans:           make([]*agentv3.SpanObject, spanSize),
		Service:         r.entity.ServiceName,
		ServiceInstance: r.entity.ServiceInstanceName,
	}
	for i, s := range spans {
		spanCtx := s.Context()
		segmentObject.Spans[i] = &agentv3.SpanObject{
			SpanId:        spanCtx.GetSpanID(),
			ParentSpanId:  spanCtx.GetParentSpanID(),
			StartTime:     s.StartTime(),
			EndTime:       s.EndTime(),
			OperationName: s.OperationName(),
			Peer:          s.Peer(),
			SpanType:      s.SpanType(),
			SpanLayer:     s.SpanLayer(),
			ComponentId:   s.ComponentID(),
			IsError:       s.IsError(),
			Tags:          s.Tags(),
			Logs:          s.Logs(),
		}
		srr := make([]*agentv3.SegmentReference, 0)
		if i == (spanSize-1) && spanCtx.GetParentSpanID() > -1 {
			srr = append(srr, &agentv3.SegmentReference{
				RefType:               agentv3.RefType_CrossThread,
				TraceId:               spanCtx.GetTraceID(),
				ParentTraceSegmentId:  spanCtx.GetParentSegmentID(),
				ParentSpanId:          spanCtx.GetParentSpanID(),
				ParentService:         r.entity.ServiceName,
				ParentServiceInstance: r.entity.ServiceInstanceName,
			})
		}
		if len(s.Refs()) > 0 {
			for _, tc := range s.Refs() {
				srr = append(srr, &agentv3.SegmentReference{
					RefType:                  agentv3.RefType_CrossProcess,
					TraceId:                  spanCtx.GetTraceID(),
					ParentTraceSegmentId:     tc.GetParentSegmentID(),
					ParentSpanId:             tc.GetParentSpanID(),
					ParentService:            tc.GetParentService(),
					ParentServiceInstance:    tc.GetParentServiceInstance(),
					ParentEndpoint:           tc.GetParentEndpoint(),
					NetworkAddressUsedAtPeer: tc.GetAddressUsedAtClient(),
				})
			}
		}
		segmentObject.Spans[i].Refs = srr
	}
	return segmentObject
}

func (r *Transform) TransformMeterData(metrics []ReportedMeter) []*agentv3.MeterData {
	if len(metrics) == 0 {
		return nil
	}
	meters := make([]*agentv3.MeterData, len(metrics))
	for i, m := range metrics {
		meter := &agentv3.MeterData{}
		switch data := m.(type) {
		case ReportedMeterSingleValue:
			meter.Metric = &agentv3.MeterData_SingleValue{
				SingleValue: &agentv3.MeterSingleValue{
					Name:   data.Name(),
					Labels: r.transformLabels(data.Labels()),
					Value:  data.Value(),
				},
			}
		case ReportedMeterHistogram:
			buckets := make([]*agentv3.MeterBucketValue, len(data.BucketValues()))
			for i, b := range data.BucketValues() {
				buckets[i] = &agentv3.MeterBucketValue{
					Bucket:             b.Bucket(),
					Count:              b.Count(),
					IsNegativeInfinity: b.IsNegativeInfinity(),
				}
			}
			meter.Metric = &agentv3.MeterData_Histogram{
				Histogram: &agentv3.MeterHistogram{
					Name:   data.Name(),
					Labels: r.transformLabels(data.Labels()),
					Values: buckets,
				},
			}
		}

		meters[i] = meter
	}

	meters[0].Service = r.entity.ServiceName
	meters[0].ServiceInstance = r.entity.ServiceInstanceName
	meters[0].Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	return meters
}

func (r *Transform) transformLabels(labels map[string]string) []*agentv3.Label {
	if len(labels) == 0 {
		return nil
	}
	ls := make([]*agentv3.Label, 0)
	for k, v := range labels {
		ls = append(ls, &agentv3.Label{
			Name:  k,
			Value: v,
		})
	}
	return ls
}
