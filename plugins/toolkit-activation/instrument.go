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

package traceactivation

import (
	"embed"

	"github.com/apache/skywalking-go/plugins/core/instrument"
)

//go:embed *
var fs embed.FS

//skywalking:nocopy
type Instrument struct {
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "toolkit-activation"
}

func (i *Instrument) BasePackage() string {
	return "github.com/apache/skywalking-go/toolkit"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

func (i *Instrument) Points() []*instrument.Point {
	var instPoints []*instrument.Point
	// append toolkit/trace related enhancements Point
	instPoints = append(instPoints, tracePoint()...)

	// append toolkit/logging related enhancements Point
	instPoints = append(instPoints, loggingPoint()...)

	// append toolkit/metric related enhancements Point
	instPoints = append(instPoints, metricPoint()...)

	return instPoints
}

func metricPoint() []*instrument.Point {
	return []*instrument.Point{
		// Counter metric type related enhancement point
		{
			PackagePath: "metric", At: instrument.NewStructEnhance("CounterRef"),
		},
		{
			PackagePath: "metric", At: instrument.NewStaticMethodEnhance("NewCounter"),
			Interceptor: "NewCounterInterceptor",
		},
		{
			PackagePath: "metric", At: instrument.NewMethodEnhance("*CounterRef", "Get"),
			Interceptor: "CounterGetInterceptor",
		},
		{
			PackagePath: "metric", At: instrument.NewMethodEnhance("*CounterRef", "Inc"),
			Interceptor: "CounterIncInterceptor",
		},
		// Gauge metric type related enhancement point
		{
			PackagePath: "metric", At: instrument.NewStructEnhance("GaugeRef"),
		},
		{
			PackagePath: "metric", At: instrument.NewStaticMethodEnhance("NewGauge"),
			Interceptor: "NewGaugeInterceptor",
		},
		{
			PackagePath: "metric", At: instrument.NewMethodEnhance("*GaugeRef", "Get"),
			Interceptor: "GaugeGetInterceptor",
		},
		// Histogram metric type related enhancement point
		{
			PackagePath: "metric", At: instrument.NewStructEnhance("HistogramRef"),
		},
		{
			PackagePath: "metric", At: instrument.NewStaticMethodEnhance("NewHistogram"),
			Interceptor: "NewHistogramInterceptor",
		},
		{
			PackagePath: "metric", At: instrument.NewMethodEnhance("*HistogramRef", "Observe"),
			Interceptor: "HistogramObserveInterceptor",
		},
		{
			PackagePath: "metric", At: instrument.NewMethodEnhance("*HistogramRef", "ObserveWithCount"),
			Interceptor: "HistogramObserveWithCountInterceptor",
		},
		// metric options related enhancement point
		{
			PackagePath: "metric", At: instrument.NewStaticMethodEnhance("WithLabels"),
			Interceptor: "WithLabelsInterceptor",
		},
	}
}

func tracePoint() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "trace", At: instrument.NewStructEnhance("SpanRef"),
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("CreateEntrySpan"),
			Interceptor: "CreateEntrySpanInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("CreateLocalSpan"),
			Interceptor: "CreateLocalSpanInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("CreateExitSpan"),
			Interceptor: "CreateExitSpanInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("StopSpan"),
			Interceptor: "StopSpanInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("CaptureContext"),
			Interceptor: "CaptureContextInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("ContinueContext"),
			Interceptor: "ContinueContextInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("GetTraceID"),
			Interceptor: "GetTraceIDInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("GetSegmentID"),
			Interceptor: "GetSegmentIDInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("GetSpanID"),
			Interceptor: "GetSpanIDInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewMethodEnhance("*SpanRef", "SetTag"),
			Interceptor: "AsyncTagInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewMethodEnhance("*SpanRef", "AddLog"),
			Interceptor: "AsyncLogInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewMethodEnhance("*SpanRef", "AddEvent"),
			Interceptor: "AsyncAddEventInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("AddEvent"),
			Interceptor: "AddEventInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("AddLog"),
			Interceptor: "AddLogInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("SetTag"),
			Interceptor: "SetTagInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("SetOperationName"),
			Interceptor: "SetOperationNameInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewMethodEnhance("*SpanRef", "PrepareAsync"),
			Interceptor: "PrepareAsyncInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewMethodEnhance("*SpanRef", "AsyncFinish"),
			Interceptor: "AsyncFinishInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("GetCorrelation"),
			Interceptor: "GetCorrelationInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("SetCorrelation"),
			Interceptor: "SetCorrelationInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("SetComponent"),
			Interceptor: "SetComponentInterceptor",
		},
		{
			PackagePath: "trace", At: instrument.NewStaticMethodEnhance("Error"),
			Interceptor: "ErrorIntercepter",
		},
	}
}

func loggingPoint() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "logging", At: instrument.NewStaticMethodEnhance("Debug"),
			Interceptor: "DebugEntryInterceptor",
		},
		{
			PackagePath: "logging", At: instrument.NewStaticMethodEnhance("Info"),
			Interceptor: "InfoEntryInterceptor",
		},
		{
			PackagePath: "logging", At: instrument.NewStaticMethodEnhance("Warn"),
			Interceptor: "WarnEntryInterceptor",
		},
		{
			PackagePath: "logging", At: instrument.NewStaticMethodEnhance("Error"),
			Interceptor: "ErrorEntryInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
