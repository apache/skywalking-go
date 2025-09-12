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
	"runtime/pprof"
	"sort"
	"strings"
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

//go:linkname runtimeSetProfLabel runtime/pprof.runtime_setProfLabel
func runtimeSetProfLabel(label unsafe.Pointer)

func (m *ProfileManager) GetPprofLabelSet(segmentID string) interface{} {
	pl := m.traceLabelSet(segmentID)
	if pl == nil {
		return &LabelSet{}
	}
	return pl
}

func (m *ProfileManager) TurnToPprofLabel(l interface{}) interface{} {
	li := l.(*LabelSet).List()
	if len(li) == 0 {
		return pprof.LabelSet{}
	}
	re := pprof.Labels(li...)
	return re
}

func UpdateTraceLabels(s *LabelSet, args ...string) *LabelSet {
	if len(args)%2 != 0 {
		panic("uneven number of arguments to profile.UpdateTraceLabels")
	}

	// add first
	for i := 0; i < len(args); i += 2 {
		s.list = append(s.list, label{key: args[i], value: args[i+1]})
	}

	// sort
	sort.SliceStable(s.list, func(i, j int) bool {
		return strings.Compare(s.list[i].key, s.list[j].key) < 0
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
		ret = append(ret, v.key)
		ret = append(ret, v.value)
	}
	return ret
}

func SetGoroutineLabels(s *LabelSet) {
	runtimeSetProfLabel(unsafe.Pointer(s))
}

func (s *LabelSet) IsEmpty() bool {
	return len(s.list) == 0
}
