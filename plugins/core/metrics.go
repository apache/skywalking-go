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
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/apache/skywalking-go/plugins/core/reporter"
)

func (t *Tracer) Metrics() interface{} {
	return t
}

func (t *Tracer) initMetricsCollect(meterCollectSecond int) {
	collectDuration := time.Duration(meterCollectSecond) * time.Second
	go func() {
		for {
			time.Sleep(collectDuration)

			t.reachNotInitMetrics()

			t.sendMetrics()
		}
	}()
}

func (t *Tracer) reachNotInitMetrics() {
	registers, hooks := MetricsObtain()
	if len(registers) == 0 && len(hooks) == 0 {
		return
	}
	for _, meter := range registers {
		switch m := meter.(type) {
		case NoInitCounter:
			counter := newCounter(m.Name(), m.Labels(), m.Get())
			m.ChangeFunctions(counter.Inc, counter.Get)
			t.registerMetrics(m.Name(), m.Labels(), counter)
		case NoInitGauge:
			gauge := newGauge(m.Name(), m.Labels(), m.Getter())
			t.registerMetrics(m.Name(), m.Labels(), gauge)
		case NoInitHistogram:
			histogram := newHistogramFromExistingBuckets(m.Name(), m.Labels(), m.Buckets())
			m.ChangeFunctions(histogram.Observe, histogram.ObserveWithCount)
			t.registerMetrics(m.Name(), m.Labels(), histogram)
		}
	}
	for _, hook := range hooks {
		t.AddCollectHook(hook)
	}
}

func (t *Tracer) sendMetrics() {
	meters := make([]reporter.ReportedMeter, 0)
	// call collect hook
	for _, hook := range t.meterCollectListeners {
		hook()
	}
	t.meterMap.Range(func(key, value interface{}) bool {
		if m, ok := value.(reporter.ReportedMeter); ok {
			meters = append(meters, m)
		} else {
			t.Log.Errorf("unknown meter type: %T", value)
		}
		return true
	})

	t.Reporter.SendMetrics(meters)
}

func (t *Tracer) NewCounter(name string, opt interface{}) interface{} {
	counter := newCounter(name, nil, 0)
	if o, ok := opt.(meterOpts); ok && o != nil {
		counter.labels = o.GetLabels()
	}
	t.registerMetrics(name, counter.labels, counter)
	return counter
}

func (t *Tracer) NewGauge(name string, getter func() float64, opt interface{}) interface{} {
	gauge := newGauge(name, nil, getter)
	if o, ok := opt.(meterOpts); ok && o != nil {
		gauge.labels = o.GetLabels()
	}
	t.registerMetrics(name, gauge.labels, gauge)
	return gauge
}

func (t *Tracer) NewHistogram(name string, minValue float64, steps []float64, opt interface{}) interface{} {
	histogram := newHistogramFromSteps(name, nil, minValue, steps)
	if o, ok := opt.(meterOpts); ok && o != nil {
		histogram.labels = o.GetLabels()
	}
	t.registerMetrics(name, histogram.labels, histogram)
	return histogram
}

func (t *Tracer) AddCollectHook(f func()) {
	t.meterCollectListeners = append(t.meterCollectListeners, f)
}

func (t *Tracer) registerMetrics(name string, labels map[string]string, meter interface{}) {
	var sb strings.Builder
	for k, v := range labels {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(v)
		sb.WriteString(",")
	}
	sb.WriteString(name)

	t.meterMap.Store(sb.String(), meter)
}

type meterOpts interface {
	GetLabels() map[string]string
}

type counterImpl struct {
	name   string
	labels map[string]string

	valBits uint64
	valInt  uint64
}

func newCounter(name string, labels map[string]string, val float64) *counterImpl {
	var valBits, valInt uint64
	ival := uint64(val)
	if float64(ival) == val {
		valInt = ival
	} else {
		valBits = math.Float64bits(val)
	}

	return &counterImpl{
		name:    name,
		labels:  labels,
		valBits: valBits,
		valInt:  valInt,
	}
}

func (c *counterImpl) Name() string {
	return c.name
}

func (c *counterImpl) Labels() map[string]string {
	return c.labels
}

func (c *counterImpl) Value() float64 {
	return c.Get()
}

func (c *counterImpl) Get() float64 {
	fval := math.Float64frombits(atomic.LoadUint64(&c.valBits))
	ival := atomic.LoadUint64(&c.valInt)
	return fval + float64(ival)
}

func (c *counterImpl) Inc(val float64) {
	if val < 0 {
		return
	}

	ival := uint64(val)
	if float64(ival) == val {
		atomic.AddUint64(&c.valInt, ival)
		return
	}

	for {
		oldBits := atomic.LoadUint64(&c.valBits)
		newBits := math.Float64bits(math.Float64frombits(oldBits) + val)
		if atomic.CompareAndSwapUint64(&c.valBits, oldBits, newBits) {
			return
		}
	}
}

type gaugeImpl struct {
	name   string
	labels map[string]string
	getter func() float64
}

func newGauge(name string, labels map[string]string, getter func() float64) *gaugeImpl {
	return &gaugeImpl{
		name:   name,
		labels: labels,
		getter: getter,
	}
}

func (g *gaugeImpl) Name() string {
	return g.name
}

func (g *gaugeImpl) Labels() map[string]string {
	return g.labels
}

func (g *gaugeImpl) Get() float64 {
	return g.getter()
}

func (g *gaugeImpl) Value() float64 {
	return g.getter()
}

type histogramImpl struct {
	name   string
	labels map[string]string

	buckets []*histogramBucket
}

func (h *histogramImpl) Name() string {
	return h.name
}

func (h *histogramImpl) Labels() map[string]string {
	return h.labels
}

func (h *histogramImpl) BucketValues() []reporter.ReportedMeterBucketValue {
	var values []reporter.ReportedMeterBucketValue
	for _, b := range h.buckets {
		values = append(values, b)
	}
	return values
}

func (h *histogramImpl) Observe(v float64) {
	if b := h.findBucket(v); b != nil {
		atomic.AddInt64(b.value, 1)
	}
}

func (h *histogramImpl) ObserveWithCount(v float64, c int64) {
	if b := h.findBucket(v); b != nil {
		atomic.AddInt64(b.value, c)
	}
}

func (h *histogramImpl) findBucket(v float64) *histogramBucket {
	var low, high = 0, len(h.buckets) - 1

	for low <= high {
		var mid = (low + high) / 2
		if h.buckets[mid].bucket < v {
			low = mid + 1
		} else if h.buckets[mid].bucket > v {
			high = mid - 1
		} else {
			return h.buckets[mid]
		}
	}

	low--

	if low < len(h.buckets) && low >= 0 {
		return h.buckets[low]
	}
	return nil
}

type histogramBucket struct {
	bucket float64
	value  *int64
}

func newHistogramFromExistingBuckets(name string, labels map[string]string, buckets []interface{}) *histogramImpl {
	result := &histogramImpl{
		name:   name,
		labels: labels,
	}

	result.buckets = make([]*histogramBucket, 0, len(buckets))
	for i, b := range buckets {
		bucket := b.(NoInitHistogramBucket)
		result.buckets[i] = &histogramBucket{
			bucket: bucket.Bucket(),
			value:  bucket.Value(),
		}
	}

	return result
}

func newHistogramFromSteps(name string, labels map[string]string, minVal float64, steps []float64) *histogramImpl {
	result := &histogramImpl{
		name:   name,
		labels: labels,
	}

	// sort steps and check for duplicates
	sort.Float64s(steps)
	prevVal := steps[0]
	for i := 1; i < len(steps); i++ {
		if prevVal == steps[i] {
			panic("duplicate histogram bucket value")
		} else {
			prevVal = steps[i]
		}
	}
	if minVal != steps[0] {
		steps = append([]float64{minVal}, steps...)
	}

	result.buckets = make([]*histogramBucket, len(steps))
	for i, s := range steps {
		result.buckets[i] = &histogramBucket{
			bucket: s,
			value:  new(int64),
		}
	}

	return result
}

func (h *histogramBucket) Bucket() float64 {
	return h.bucket
}

func (h *histogramBucket) Count() int64 {
	return *h.value
}

func (h *histogramBucket) IsNegativeInfinity() bool {
	return false
}

type NoInitCounter interface {
	Name() string
	Labels() map[string]string
	Get() float64
	ChangeFunctions(add func(v float64), get func() float64)
}

type NoInitGauge interface {
	Name() string
	Labels() map[string]string
	Getter() func() float64
}

type NoInitHistogram interface {
	Name() string
	Labels() map[string]string
	Buckets() []interface{}
	ChangeFunctions(observe func(v float64), observeWithCount func(v float64, c int64))
}

type NoInitHistogramBucket interface {
	Bucket() float64
	Value() *int64
}
