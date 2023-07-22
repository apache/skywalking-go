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

func newMeterOpts() *Opts {
	return &Opts{labels: make(map[string]string)}
}

func (o *Opts) GetLabels() map[string]string {
	return o.labels
}

type counterImpl struct {
	name   string
	val    float64
	labels map[string]string

	getFunc func() float64
	addFunc func(float64)
}

func newDefaultCounter(name string, opt ...Opt) *counterImpl {
	result := &counterImpl{name: name, val: 0}
	opts := newMeterOpts()
	for _, o := range opt {
		o(opts)
	}
	result.labels = opts.GetLabels()

	result.getFunc = func() float64 {
		return result.val
	}
	result.addFunc = func(f float64) {
		result.val += f
	}
	return result
}

func (c *counterImpl) Name() string {
	return c.name
}

func (c *counterImpl) Get() float64 {
	return c.getFunc()
}

func (c *counterImpl) Labels() map[string]string {
	return c.labels
}

func (c *counterImpl) Inc(val float64) {
	c.addFunc(val)
}

func (c *counterImpl) ChangeFunctions(add func(v float64), get func() float64) {
	c.addFunc = add
	c.getFunc = get
}

type gaugeImpl struct {
	name   string
	labels map[string]string
	getter func() float64
}

func newDefaultGauge(name string, getter func() float64, opt ...Opt) *gaugeImpl {
	result := &gaugeImpl{name: name, getter: getter}
	opts := newMeterOpts()
	for _, o := range opt {
		o(opts)
	}
	result.labels = opts.GetLabels()
	return result
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

func (g *gaugeImpl) Getter() func() float64 {
	return g.getter
}

type histogramImpl struct {
	name   string
	labels map[string]string

	buckets []*histogramBucket

	observeFunc          func(float64)
	observeWithCountFunc func(float64, int64)
}

type histogramBucket struct {
	bucket float64
	val    *int64
}

func newDefaultHistogram(name string, minVal float64, steps []float64, opt ...Opt) *histogramImpl {
	result := &histogramImpl{name: name}
	opts := newMeterOpts()
	for _, o := range opt {
		o(opts)
	}
	result.labels = opts.GetLabels()

	result.initBuckets(minVal, steps)

	result.observeFunc = func(f float64) {
		b := result.findBucket(f)
		if b != nil {
			*b.val++
		}
	}
	result.observeWithCountFunc = func(f float64, c int64) {
		b := result.findBucket(f)
		if b != nil {
			*b.val += c
		}
	}
	return result
}

func (h *histogramImpl) Observe(val float64) {
	h.observeFunc(val)
}

func (h *histogramImpl) ObserveWithCount(val float64, c int64) {
	h.observeWithCountFunc(val, c)
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

func (h *histogramImpl) initBuckets(minVal float64, steps []float64) {
	n := len(steps)
	for i := 1; i < n; i++ {
		key := steps[i]
		j := i - 1
		if steps[j] == key {
			panic("duplicate steps found")
		}
		for j >= 0 && steps[j] > key {
			steps[j+1] = steps[j]
			j--
		}
		steps[j+1] = key
	}
	if steps[0] != minVal {
		steps = append([]float64{minVal}, steps...)
	}

	buckets := make([]*histogramBucket, len(steps))
	for i, step := range steps {
		buckets[i] = &histogramBucket{bucket: step, val: new(int64)}
	}
	h.buckets = buckets
}

func (h *histogramImpl) Name() string {
	return h.name
}

func (h *histogramImpl) Labels() map[string]string {
	return h.labels
}

func (h *histogramImpl) Buckets() []interface{} {
	result := make([]interface{}, len(h.buckets))
	for i, b := range h.buckets {
		result[i] = b
	}
	return result
}

func (h *histogramImpl) ChangeFunctions(observe func(v float64), observeWithCount func(v float64, c int64)) {
	h.observeFunc = observe
	h.observeWithCountFunc = observeWithCount
}

func (h *histogramBucket) Bucket() float64 {
	return h.bucket
}

func (h *histogramBucket) Value() *int64 {
	return h.val
}

type timerImpl struct {
	namePrefix string
	labels     map[string]string
	startFunc  func() TimerSample
}

type timerSampleImpl struct {
	// empty implementation for sample(because no time package import)
}

func (t *timerSampleImpl) Stop() {
}

func (t *timerSampleImpl) Duration() int64 {
	return 0
}

func NewDefaultTimer(namePrefix string, opt ...Opt) Timer {
	opts := newMeterOpts()
	for _, o := range opt {
		o(opts)
	}
	result := &timerImpl{namePrefix: namePrefix, labels: opts.labels}
	result.startFunc = func() TimerSample {
		return &timerSampleImpl{}
	}
	return result
}

func NewAgentCoreTimer(timer agentTimer) Timer {
	return &timerImpl{
		startFunc: func() TimerSample {
			return timer.Start().(TimerSample)
		},
	}
}

func (t *timerImpl) Start() TimerSample {
	return t.startFunc()
}

func (t *timerImpl) NamePrefix() string {
	return t.namePrefix
}

func (t *timerImpl) Labels() map[string]string {
	return t.labels
}

func (t *timerImpl) ChangeFunction(startTimer func() interface{}) {
	t.startFunc = func() TimerSample {
		return startTimer().(TimerSample)
	}
}

type agentTimer interface {
	Start() interface{}
}
