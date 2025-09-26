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
	"runtime"
	"runtime/pprof"
	"sort"
	"unsafe"

	"github.com/apache/skywalking-go/plugins/core/profile"
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

//go:linkname runtimeGetProfLabel runtime/pprof.runtime_getProfLabel
func runtimeGetProfLabel() unsafe.Pointer

func GetNowLabelSet() LabelSet {
	pl := LabelSet{
		list: make([]label, 0),
	}
	p := runtimeGetProfLabel()
	if p != nil {
		version := runtime.Version()
		if version < "go1.20" {
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

func (m *ProfileManager) IsSkywalkingInternalCtx(ctx interface{}) bool {
	c := ctx.(context.Context)
	if c == nil {
		return false
	}
	if c.Value(profile.SkywalkingInternalKey) != nil {
		return true
	}
	return false
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
		pprof.SetGoroutineLabels(c)
		return
	}
	ctx := context.WithValue(context.Background(), profile.PprofContextKey{}, true)
	labels := pprof.Labels(s.List()...)
	ctx = pprof.WithLabels(ctx, labels)
	pprof.SetGoroutineLabels(ctx)
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
