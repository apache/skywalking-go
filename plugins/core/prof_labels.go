// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package core

import (
	"context"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"unsafe"
)

type label struct {
	key   string
	value string
}

type LabelSet struct {
	list []label
}

type labelMap struct {
	LabelSet
}

type labelMap19 map[string]string

type labelContextKey struct{}

//go:linkname runtimeGetProfLabel runtime/pprof.runtime_getProfLabel
func runtimeGetProfLabel() unsafe.Pointer

//go:linkname runtimeSetProfLabel runtime/pprof.runtime_setProfLabel
func runtimeSetProfLabel(label unsafe.Pointer)

func setGoroutineLabelsInternal(ctx context.Context) {
	if isGoVersionLMoreThan120(runtime.Version()) {
		ctxLabels, _ := ctx.Value(labelContextKey{}).(*labelMap)
		runtimeSetProfLabel(unsafe.Pointer(ctxLabels))
		return
	}
	ctxLabels, _ := ctx.Value(labelContextKey{}).(*labelMap19)
	runtimeSetProfLabel(unsafe.Pointer(ctxLabels))
}

func labelValue(ctx context.Context) labelMap19 {
	labels, _ := ctx.Value(labelContextKey{}).(*labelMap19)
	if labels == nil {
		return labelMap19(nil)
	}
	return *labels
}

func WithLabels(ctx context.Context, s LabelSet) context.Context {
	if isGoVersionLMoreThan120(runtime.Version()) {
		ctx = context.WithValue(ctx, labelContextKey{}, &labelMap{s})
		return ctx
	}
	return withLabels19(ctx, s)
}

func withLabels19(ctx context.Context, labels LabelSet) context.Context {
	childLabels := make(labelMap19)
	parentLabels := labelValue(ctx)
	for k, v := range parentLabels {
		childLabels[k] = v
	}
	for _, label := range labels.list {
		childLabels[label.key] = label.value
	}
	return context.WithValue(ctx, labelContextKey{}, &childLabels)
}

func GetNowLabelSet() LabelSet {
	pl := LabelSet{
		list: make([]label, 0),
	}
	p := runtimeGetProfLabel()
	if p != nil {
		version := runtime.Version()
		if !isGoVersionLMoreThan120(version) {
			// Go1.19：map[string]string -> []label
			m := *(*labelMap19)(p)
			pl.list = make([]label, 0, len(m))
			for k, v := range m {
				pl.list = append(pl.list, label{key: k, value: v})
			}
		} else {
			lm := (*labelMap)(p)
			pl.list = lm.list
		}
	}
	return pl
}

// isGoVersionLMoreThan120 parses version strings like "go1.19.8"
func isGoVersionLMoreThan120(version string) bool {
	re := regexp.MustCompile(`go(\d+)\.(\d+)`)
	sub := re.FindStringSubmatch(version)
	if len(sub) != 3 {
		return false
	}
	major, err1 := strconv.Atoi(sub[1])
	minor, err2 := strconv.Atoi(sub[2])
	if err1 != nil || err2 != nil {
		return false
	}
	if major < 1 {
		return false
	}
	if major > 1 {
		return true
	}
	return minor >= 20
}

func (m *ProfileManager) AddSkyLabels(traceID, segmentID string, spanID int32) interface{} {
	pl := GetNowLabelSet()
	re := UpdateTraceLabels(pl, TraceLabel, traceID, SegmentLabel, segmentID, SpanLabel, parseString(spanID))
	return &re
}

func (m *ProfileManager) TurnToPprofLabel(l interface{}) interface{} {
	li := l.(*LabelSet).List()
	if len(li) == 0 {
		return pprof.LabelSet{}
	}
	re := pprof.Labels(li...)
	return re
}

func UpdateTraceLabels(s LabelSet, args ...string) LabelSet {
	if len(args)%2 != 0 {
		panic("uneven number of arguments to profile.UpdateTraceLabels")
	}

	// add first
	for i := 0; i < len(args); i += 2 {
		s.list = append(s.list, label{key: args[i], value: args[i+1]})
	}

	// sort
	sort.SliceStable(s.list, func(i, j int) bool {
		return s.list[i].key < s.list[j].key
	})

	// remove duplicates
	deduped := make([]label, 0, len(s.list))
	for i, lbl := range s.list {
		if i == 0 || lbl.key != s.list[i-1].key {
			deduped = append(deduped, lbl)
		} else {
			deduped[len(deduped)-1] = lbl
		}
	}
	s.list = deduped

	return s
}

func (s *LabelSet) List() []string {
	var ret []string
	for _, v := range s.list {
		ret = append(ret, v.key, v.value)
	}
	return ret
}

func SetGoroutineLabels(s *LabelSet) {
	if s.IsEmpty() {
		var c = context.Background()
		setGoroutineLabelsInternal(c)
		return
	}
	var c = context.Background()
	l := *s
	c = WithLabels(c, l)
	setGoroutineLabelsInternal(c)
}

// GetNowLabels Expose to operator
func (m *ProfileManager) GetNowLabels() interface{} {
	re := GetNowLabelSet()
	return &re
}

func (s *LabelSet) IsEmpty() bool {
	if s == nil || s.list == nil {
		return true
	}
	return len(s.list) == 0
}
