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

package metrics

import "github.com/apache/skywalking-go/plugins/core/operator"

type Opt func(opts *Opts)

type Opts struct {
	labels map[string]string
}

// WithLabel adds a label to the metrics.
func WithLabel(key, value string) Opt {
	return func(meter *Opts) {
		meter.labels[key] = value
	}
}

type Counter interface {
	// Get returns the current value of the counter.
	Get() float64
	// Inc increments the counter with value.
	Inc(val float64)
}

type Gauge interface {
	// Get returns the current value of the gauge.
	Get() float64
}

type Histogram interface {
	// Observe find the value associate bucket and add 1.
	Observe(val float64)
	// ObserveWithCount find the value associate bucket and add specific count.
	ObserveWithCount(val float64, count int64)
}

// NewCounter creates a new counter metrics.
// name is the name of the metrics
// opts is the options for the metrics
func NewCounter(name string, opts ...Opt) Counter {
	op := operator.GetOperator()
	if op == nil {
		tmpCounter := newDefaultCounter(name, opts...)
		operator.MetricsAppender(tmpCounter)
		return tmpCounter
	}

	opt := newMeterOpts()
	for _, o := range opts {
		o(opt)
	}
	return op.Metrics().(operator.MetricsOperator).NewCounter(name, opt).(Counter)
}

// NewGauge creates a new gauge metrics.
// name is the name of the metrics
// getter is the function to get the value of the gauge meter
// opts is the options for the metrics
func NewGauge(name string, getter func() float64, opts ...Opt) Gauge {
	op := operator.GetOperator()
	if op == nil {
		tmpGauge := newDefaultGauge(name, getter, opts...)
		operator.MetricsAppender(tmpGauge)
		return tmpGauge
	}

	opt := newMeterOpts()
	for _, o := range opts {
		o(opt)
	}
	return op.Metrics().(operator.MetricsOperator).NewGauge(name, getter, opt).(Gauge)
}

// NewHistogram creates a new histogram metrics.
// name is the name of the metrics
// steps is the buckets of the histogram
// opts is the options for the metrics
func NewHistogram(name string, steps []float64, opts ...Opt) Histogram {
	return NewHistogramWithMinValue(name, 0, steps, opts...)
}

// NewHistogramWithMinValue creates a new histogram metrics.
// name is the name of the metrics
// minVal is the min value of the histogram bucket
// steps is the buckets of the histogram
// opts is the options for the metrics
func NewHistogramWithMinValue(name string, minVal float64, steps []float64, opts ...Opt) Histogram {
	op := operator.GetOperator()
	if op == nil {
		tmpHistogram := newDefaultHistogram(name, minVal, steps, opts...)
		operator.MetricsAppender(tmpHistogram)
		return tmpHistogram
	}

	opt := newMeterOpts()
	for _, o := range opts {
		o(opt)
	}
	return op.Metrics().(operator.MetricsOperator).NewHistogram(name, minVal, steps, opt).(Histogram)
}

// RegisterBeforeCollectHook registers a hook function which will be called before metrics collect.
func RegisterBeforeCollectHook(f func()) {
	op := operator.GetOperator()
	if op == nil {
		operator.MetricsCollectAppender(f)
		return
	}

	op.Metrics().(operator.MetricsOperator).AddCollectHook(f)
}
