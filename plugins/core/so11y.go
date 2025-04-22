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
	"sync"
	"time"

	"github.com/apache/skywalking-go/plugins/core/metrics"
)

var (
	instance *So11y
	once     sync.Once
)

type So11y struct {
	propagatedContextCounter       metrics.Counter
	propagatedIgnoreContextCounter metrics.Counter

	samplerContextCounter       metrics.Counter
	samplerIgnoreContextCounter metrics.Counter

	finishContextCounter       metrics.Counter
	finishIgnoreContextCounter metrics.Counter

	leakedContextCounter       metrics.Counter
	leakedIgnoreContextCounter metrics.Counter

	errorCounterMap     sync.Map
	interceptorTimeCost metrics.Histogram
}

func GetSo11y(t *Tracer) *So11y {
	once.Do(func() {
		instance = &So11y{
			propagatedIgnoreContextCounter: t.NewCounter("sw_go_created_ignored_context_counter",
				&metrics.Opts{
					Labels: map[string]string{"created_by": "propagated"},
				}).(metrics.Counter),
			propagatedContextCounter: t.NewCounter("sw_go_created_tracing_context_counter",
				&metrics.Opts{
					Labels: map[string]string{"created_by": "propagated"},
				}).(metrics.Counter),

			samplerIgnoreContextCounter: t.NewCounter("sw_go_created_ignored_context_counter",
				&metrics.Opts{
					Labels: map[string]string{"created_by": "sampler"},
				}).(metrics.Counter),
			samplerContextCounter: t.NewCounter("sw_go_created_tracing_context_counter", &metrics.Opts{
				Labels: map[string]string{"created_by": "sampler"},
			}).(metrics.Counter),

			finishIgnoreContextCounter: t.NewCounter(
				"sw_go_finished_ignored_context_counter", nil).(metrics.Counter),
			finishContextCounter: t.NewCounter(
				"sw_go_finished_tracing_context_counter", nil).(metrics.Counter),

			leakedIgnoreContextCounter: t.NewCounter("sw_go_possible_leaked_context_counter",
				&metrics.Opts{
					Labels: map[string]string{"source": "ignore"},
				}).(metrics.Counter),
			leakedContextCounter: t.NewCounter("sw_go_possible_leaked_context_counter",
				&metrics.Opts{
					Labels: map[string]string{"created_by": "tracing"},
				}).(metrics.Counter),

			interceptorTimeCost: t.NewHistogram("sw_go_tracing_context_performance", 0,
				[]float64{
					1000, 10000, 50000, 100000, 300000, 500000,
					1000000, 5000000, 10000000, 20000000, 50000000, 100000000,
				}, nil).(metrics.Histogram),
		}
	})
	return instance
}

func (s *So11y) MeasureTracingContextCreation(isForceSample, isIgnored bool) {
	if isForceSample {
		if isIgnored {
			s.propagatedIgnoreContextCounter.Inc(1)
		} else {
			s.propagatedContextCounter.Inc(1)
		}
	} else {
		if isIgnored {
			s.samplerIgnoreContextCounter.Inc(1)
		} else {
			s.samplerContextCounter.Inc(1)
		}
	}
}

func (s *So11y) MeasureTracingContextCompletion(isIgnored bool) {
	if isIgnored {
		s.finishIgnoreContextCounter.Inc(1)
	} else {
		s.finishContextCounter.Inc(1)
	}
}

func (s *So11y) MeasureLeakedTracingContext(isIgnored bool) {
	if isIgnored {
		s.leakedIgnoreContextCounter.Inc(1)
	} else {
		s.leakedContextCounter.Inc(1)
	}
}

func (t *Tracer) So11y() interface{} {
	return t
}

func (t *Tracer) CollectErrorOfPlugin(pluginName string) {
	if counter, ok := GetSo11y(t).errorCounterMap.Load(pluginName); ok {
		if c, ok := counter.(metrics.Counter); ok {
			c.Inc(1)
			return
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if counter, ok := GetSo11y(t).errorCounterMap.Load(pluginName); ok {
		if c, ok := counter.(metrics.Counter); ok {
			c.Inc(1)
			return
		}
	}

	if counter, ok := t.NewCounter(
		"sw_go_interceptor_error_counter", &metrics.Opts{
			Labels: map[string]string{"plugin_name": pluginName},
		}).(metrics.Counter); ok {
		GetSo11y(t).errorCounterMap.Store(pluginName, counter)
		counter.Inc(1)
	}
}

func (t *Tracer) GenNanoTime() int64 {
	return time.Now().UnixNano()
}

func (t *Tracer) CollectDurationOfInterceptor(costTime int64) {
	GetSo11y(t).interceptorTimeCost.Observe(float64(costTime))
}
