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
	"testing"

	"github.com/stretchr/testify/assert"
)

func sortLabels(ls LabelSet) LabelSet {
	sort.Slice(ls.list, func(i, j int) bool {
		return ls.list[i].key < ls.list[j].key
	})
	return ls
}

func TestGetLabels(t *testing.T) {
	p := NewProfileManager(nil)
	p.currentTask = &currentTask{
		serialNumber:         "",
		taskID:               "",
		minDurationThreshold: 0,
		endpointName:         "",
		duration:             0,
	}
	ls := p.GetPprofLabelSet("test-TraceID", "test-segmentID", 0).(*LabelSet)
	ts := LabelSet{list: []label{
		{key: "spanID", value: "0"},
		{key: "traceSegmentID", value: "test-segmentID"},
		{key: "traceID", value: "test-TraceID"}}}
	assert.Equal(t, sortLabels(ts), sortLabels(*ls))
}

func TestTurnToPprofLabel(t *testing.T) {
	p := NewProfileManager(nil)
	// test Label have nothing
	re1 := p.TurnToPprofLabel(&LabelSet{}).(pprof.LabelSet)
	assert.Equal(t, re1, pprof.LabelSet{})

	// test Label have something
	re2 := p.TurnToPprofLabel(&LabelSet{list: []label{
		{key: "minDurationThreshold", value: "0"},
		{key: "spanID", value: "0"},
		{key: "traceSegmentID", value: "test-segmentID"}}}).(pprof.LabelSet)
	l2 := pprof.Labels("minDurationThreshold", "0", "spanID", "0", "traceSegmentID", "test-segmentID")
	assert.Equal(t, re2, l2)
}
