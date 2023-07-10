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

package runtimemetrics

import (
	"math"
	original_metrics "runtime/metrics"

	"github.com/apache/skywalking-go/plugins/core/metrics"
)

//nolint
var nameReplacing = map[string]*meterInfo{
	// GC counts
	"/gc/cycles/automatic:gc-cycles": newMetricsGaugeReplaceInfo("instance_golang_gc_count_labeled", "type", "automatic"),
	"/gc/cycles/forced:gc-cycles":    newMetricsGaugeReplaceInfo("instance_golang_gc_count_labeled", "type", "forced"),
	"/gc/cycles/total:gc-cycles":     newMetricsGaugeReplaceInfo("instance_golang_gc_count_labeled", "type", "total"),

	// Heap allocs
	"/gc/heap/allocs:bytes":   newMetricsGaugeReplaceInfo("instance_golang_heap_alloc_size"),
	"/gc/heap/allocs:objects": newMetricsGaugeReplaceInfo("instance_golang_heap_alloc_objects"),

	// Heap frees
	"/gc/heap/frees:bytes":   newMetricsGaugeReplaceInfo("instance_golang_heap_frees"),
	"/gc/heap/frees:objects": newMetricsGaugeReplaceInfo("instance_golang_heap_frees_objects"),

	// Memory heap
	"/memory/classes/heap/free:bytes":     newMetricsGaugeReplaceInfo("instance_golang_memory_heap_labeled", "type", "free"),
	"/memory/classes/heap/objects:bytes":  newMetricsGaugeReplaceInfo("instance_golang_memory_heap_labeled", "type", "objects"),
	"/memory/classes/heap/released:bytes": newMetricsGaugeReplaceInfo("instance_golang_memory_heap_labeled", "type", "released"),
	"/memory/classes/heap/stacks:bytes":   newMetricsGaugeReplaceInfo("instance_golang_memory_heap_labeled", "type", "stacks"),
	"/memory/classes/heap/unused:bytes":   newMetricsGaugeReplaceInfo("instance_golang_memory_heap_labeled", "type", "unused"),

	// Metadata mcache
	"/memory/classes/metadata/mcache/free:bytes":  newMetricsGaugeReplaceInfo("instance_golang_metadata_mcache_labeled", "type", "free"),
	"/memory/classes/metadata/mcache/inuse:bytes": newMetricsGaugeReplaceInfo("instance_golang_metadata_mcache_labeled", "type", "inuse"),

	// Metadata mspan
	"/memory/classes/metadata/mspan/free:bytes":  newMetricsGaugeReplaceInfo("instance_golang_metadata_mspan_labeled", "type", "free"),
	"/memory/classes/metadata/mspan/inuse:bytes": newMetricsGaugeReplaceInfo("instance_golang_metadata_mspan_labeled", "type", "inuse"),

	// threads
	"/sched/gomaxprocs:threads":    newMetricsGaugeReplaceInfo("instance_golang_os_threads_num"),
	"/sched/goroutines:goroutines": newMetricsGaugeReplaceInfo("instance_golang_live_goroutines_num"),

	// Others
	"/cgo/go-to-c-calls:calls":                newMetricsGaugeReplaceInfo("instance_golang_cgo_calls"),
	"/gc/heap/goal:bytes":                     newMetricsGaugeReplaceInfo("instance_golang_gc_heap_goal"),
	"/gc/heap/objects:objects":                newMetricsGaugeReplaceInfo("instance_golang_gc_heap_objects"),
	"/gc/heap/tiny/allocs:objects":            newMetricsGaugeReplaceInfo("instance_golang_gc_heap_tiny_allocs"),
	"/gc/limiter/last-enabled:gc-cycle":       newMetricsGaugeReplaceInfo("instance_golang_gc_limiter_last_enabled"),
	"/gc/stack/starting-size:bytes":           newMetricsGaugeReplaceInfo("instance_golang_gc_stack_starting_size"),
	"/memory/classes/metadata/other:bytes":    newMetricsGaugeReplaceInfo("instance_golang_memory_metadata_other"),
	"/memory/classes/os-stacks:bytes":         newMetricsGaugeReplaceInfo("instance_golang_memory_os_stacks"),
	"/memory/classes/other:bytes":             newMetricsGaugeReplaceInfo("instance_golang_memory_other"),
	"/memory/classes/profiling/buckets:bytes": newMetricsGaugeReplaceInfo("instance_golang_memory_profiling_buckets"),
	"/memory/classes/total:bytes":             newMetricsGaugeReplaceInfo("instance_golang_memory_total"),

	// Histogram
	"/gc/heap/allocs-by-size:bytes": newMetricsHistogramReplaceInfo("instance_golang_gc_heap_allocs_by_size", 1),
	"/gc/heap/frees-by-size:bytes":  newMetricsHistogramReplaceInfo("instance_golang_gc_heap_frees_by_size", 1),
	"/gc/pauses:seconds":            newMetricsHistogramReplaceInfo("instance_golang_gc_pauses", 1000_000_000),
	"/sched/latencies:seconds":      newMetricsHistogramReplaceInfo("instance_golang_sched_latencies", 1000_000_000),
}

//nolint
var combinedMetrics = []*meterInfo{
	newCombinedGaugeInfo("instance_golang_memory_heap_labeled", []string{
		"/memory/classes/heap/free:bytes", "/memory/classes/heap/objects:bytes", "/memory/classes/heap/released:bytes",
		"/memory/classes/heap/stacks:bytes", "/memory/classes/heap/unused:bytes",
	}, "type", "total"),
}

//nolint
//skywalking:init
func registerMetrics() {
	allMetrics := original_metrics.All()
	samples := make([]original_metrics.Sample, 0)
	infos := make(map[string]*meterInfo)
	combinedInfos := make([]*meterInfo, 0)

	for _, m := range allMetrics {
		info := nameReplacing[m.Name]
		if info == nil {
			continue
		}

		sample := original_metrics.Sample{Name: m.Name}
		samples = append(samples, sample)
		info.init(sample)
		infos[m.Name] = info
	}

	for _, info := range combinedMetrics {
		info.initWithCombined()
		combinedInfos = append(combinedInfos, info)
	}

	metrics.RegisterBeforeCollectHook(func() {
		// reading all samples
		original_metrics.Read(samples)
		// updating metrics
		for _, sample := range samples {
			if i := infos[sample.Name]; i != nil {
				i.updateMetricValue(sample)
			}
		}
		for _, info := range combinedInfos {
			info.updateCombinedMetricValue(samples)
		}
	})
}

type meterInfo struct {
	// basic info
	name              string
	tagOpts           []metrics.Opt
	isHistogram       bool
	histogramMultiple int

	// metric value
	gaugeValue           float64
	gaugeMetric          metrics.Gauge
	latestHistogramValue []int64
	histogramMetric      metrics.Histogram
	histogramStartInx    int

	// combined metrics
	needsMetricsNames   []string
	combinedGaugeMetric metrics.Gauge
	combinedGaugeValue  float64
}

func newMetricsGaugeReplaceInfo(name string, tags ...string) *meterInfo {
	meter := &meterInfo{name: name, isHistogram: false}
	if len(tags) > 0 {
		meter.tagOpts = make([]metrics.Opt, 0)
		for i := 0; i < len(tags); i += 2 {
			meter.tagOpts = append(meter.tagOpts, metrics.WithLabel(tags[i], tags[i+1]))
		}
	}
	return meter
}

func newMetricsHistogramReplaceInfo(name string, multiples int) *meterInfo {
	meter := &meterInfo{name: name, isHistogram: true, histogramMultiple: multiples}
	return meter
}

func newCombinedGaugeInfo(name string, needsMetrics []string, tags ...string) *meterInfo {
	meter := &meterInfo{name: name, isHistogram: false, needsMetricsNames: needsMetrics}
	if len(tags) > 0 {
		meter.tagOpts = make([]metrics.Opt, 0)
		for i := 0; i < len(tags); i += 2 {
			meter.tagOpts = append(meter.tagOpts, metrics.WithLabel(tags[i], tags[i+1]))
		}
	}
	return meter
}

func (m *meterInfo) init(sample original_metrics.Sample) {
	if !m.isHistogram {
		m.gaugeMetric = metrics.NewGauge(m.name, func() float64 {
			return m.gaugeValue
		}, m.tagOpts...)
		return
	}

	original_metrics.Read([]original_metrics.Sample{sample})
	m.initHistogramIfNeeds(sample)
}

func (m *meterInfo) initWithCombined() {
	m.combinedGaugeMetric = metrics.NewGauge(m.name, func() float64 {
		return m.combinedGaugeValue
	}, m.tagOpts...)
}

func (m *meterInfo) updateMetricValue(sample original_metrics.Sample) {
	if !m.isHistogram {
		if v, ok := m.readingFloat64(sample); ok {
			m.gaugeValue = v
		}
		return
	}

	m.initHistogramIfNeeds(sample)
	if m.histogramMetric == nil || m.latestHistogramValue == nil {
		return
	}
	histogram, ok := m.readingHistogram(sample)
	if !ok {
		return
	}
	for i, val := range histogram.Counts {
		if i < m.histogramStartInx {
			continue
		}
		if i >= len(m.latestHistogramValue) {
			break
		}
		newestValue := int64(val)
		if add := newestValue - m.latestHistogramValue[i]; add > 0 {
			m.histogramMetric.ObserveWithCount(histogram.Buckets[i]*float64(m.histogramMultiple), add)
			m.latestHistogramValue[i] = newestValue
		}
	}
}

func (m *meterInfo) initHistogramIfNeeds(sample original_metrics.Sample) {
	if m.histogramMetric != nil {
		return
	}
	histogram, ok := m.readingHistogram(sample)
	if !ok {
		return
	}
	float64s := make([]float64, 0, len(histogram.Buckets))
	val := make([]int64, 0, len(histogram.Buckets))
	var startInx = 0
	var hasAdded = false
	for _, b := range histogram.Buckets {
		if b > math.MaxFloat64 || b < 0 {
			if !hasAdded {
				startInx++
			}
			continue
		}
		hasAdded = true
		float64s = append(float64s, b*float64(m.histogramMultiple))
		val = append(val, 0)
	}
	m.histogramStartInx = startInx
	m.histogramMetric = metrics.NewHistogram(m.name, float64s, m.tagOpts...)
	m.latestHistogramValue = val
}

func (m *meterInfo) readingFloat64(s original_metrics.Sample) (float64, bool) {
	switch s.Value.Kind() {
	case original_metrics.KindUint64:
		return float64(s.Value.Uint64()), true
	case original_metrics.KindFloat64:
		return s.Value.Float64(), true
	default:
		return 0, false
	}
}

func (m *meterInfo) readingHistogram(s original_metrics.Sample) (*original_metrics.Float64Histogram, bool) {
	if s.Value.Kind() != original_metrics.KindFloat64Histogram {
		return nil, false
	}
	return s.Value.Float64Histogram(), true
}

func (m *meterInfo) updateCombinedMetricValue(samples []original_metrics.Sample) {
	var sum float64
	for _, name := range m.needsMetricsNames {
		for _, sample := range samples {
			if sample.Name == name {
				if v, ok := m.readingFloat64(sample); ok {
					sum += v
				}
				break
			}
		}
	}
	m.combinedGaugeValue = sum
}
