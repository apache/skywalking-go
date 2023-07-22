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

var durationHistogramSteps = []float64{
	0, 1, 3, 5, 7, 9, 10, 13, 17, 22, 28, 35, 43, 52, 62, 73, 85, 98, 112, 127, 143, 160, 178, 200, 223, 247, 273, 300,
	400, 500, 600, 700, 800, 900, 1000, 1200, 1400, 1600, 2000, 3000, 5000, 10000, 20000, 30000, 40000, 50000, 60000,
	70000, 80000, 90000, 100000, 200000, 300000, 400000, 500000, 600000, 700000, 800000, 900000, 1000000, 2000000,
	3000000, 4000000, 5000000, 6000000, 7000000, 8000000, 9000000, 10000000, 13000000, 16000000, 19000000, 22000000,
	25000000, 28000000, 31000000, 34000000, 37000000, 40000000, 43000000, 46000000, 49000000, 52000000, 55000000,
	60000000, 70000000, 80000000, 90000000, 100000000, 120000000, 140000000, 160000000, 180000000, 200000000,
	250000000, 300000000, 350000000, 400000000, 450000000, 500000000, 550000000, 600000000, 650000000, 700000000,
	750000000, 800000000, 850000000, 900000000, 950000000, 1000000000, 1100000000, 1200000000, 1300000000,
	1400000000, 1500000000, 1600000000, 1700000000, 1800000000, 1900000000, 2000000000, 2500000000, 3000000000,
}

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
		case NoInitTimer:
			timer := newTimer(m.NamePrefix(), m.Labels())
			m.ChangeFunction(timer.Start)
			timer.registerToTracer(t)
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

func (t *Tracer) NewTimer(namePrefix string, opts interface{}) interface{} {
	var labels map[string]string
	if o, ok := opts.(meterOpts); ok && o != nil {
		labels = o.GetLabels()
	}
	timer := newTimer(namePrefix, labels)
	timer.registerToTracer(t)
	return timer
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

type timerImpl struct {
	// for calculating avg duration
	totalCounter    *counterImpl
	durationCounter *counterImpl

	// for calculating duration histogram
	durationHistogram *histogramImpl
}

type timerSampleImpl struct {
	timer *timerImpl
	start time.Time
	end   *time.Time
}

func newTimer(namePrefix string, labels map[string]string) *timerImpl {
	return &timerImpl{
		totalCounter:    newCounter(namePrefix+"_total", labels, 0),
		durationCounter: newCounter(namePrefix+"_duration", labels, 0),
		durationHistogram: newHistogramFromSteps(namePrefix+"_duration_histogram", labels,
			0, durationHistogramSteps),
	}
}

func (t *timerImpl) registerToTracer(tracer *Tracer) {
	tracer.registerMetrics(t.totalCounter.name, t.totalCounter.labels, t.totalCounter)
	tracer.registerMetrics(t.durationCounter.name, t.durationCounter.labels, t.durationCounter)
	tracer.registerMetrics(t.durationHistogram.name, t.durationHistogram.labels, t.durationHistogram)
}

func (t *timerImpl) Start() interface{} {
	return &timerSampleImpl{
		timer: t,
		start: time.Now(),
	}
}

func (t *timerSampleImpl) Stop() {
	now := time.Now()
	t.end = &now

	// appending to the metrics
	usedDuration := float64(t.Duration())
	timer := t.timer
	timer.totalCounter.Inc(1)
	timer.durationCounter.Inc(usedDuration)
	timer.durationHistogram.Observe(usedDuration)
}

func (t *timerSampleImpl) Duration() int64 {
	if t.end == nil {
		return 0
	}
	return t.end.Sub(t.start).Nanoseconds()
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

type NoInitTimer interface {
	NamePrefix() string
	Labels() map[string]string
	ChangeFunction(startTimer func() interface{})
}
