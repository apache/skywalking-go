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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLabels(t *testing.T) {
	p := NewProfileManager(nil)
	re := p.generateProfileLabels("test-segmentID", 0)
	p.labelSets["test-segmentID"] = re
	p.AddSpanId("test-segmentID", 0)
	ls := p.GetPprofLabelSet("test-segmentID").(*LabelSet)
	ts := LabelSet{list: []label{
		{key: "minDurationThreshold", value: "0"},
		{key: "spanID", value: "0"},
		{key: "traceSegmentID", value: "test-segmentID"}}}
	assert.Equal(t, ts, *ls)
}

func TestSetLabels(t *testing.T) {
	ts := &LabelSet{list: []label{{"test1", "test1_label"}, {"test2", "test2_label"}}}
	labels := UpdateTraceLabels(ts, "test3", "test3_label")
	SetGoroutineLabels(labels)
	p := NewProfileManager(nil)
	p.labelSets["test-segmentID"] = profileLabels{
		labels: labels,
	}
	re := p.GetPprofLabelSet("test-segmentID").(*LabelSet)
	assert.Equal(t, re, ts)
}

func TestTurnToPprofLabel(t *testing.T) {
	p := NewProfileManager(nil)
	// test Label have nothing
	re1 := p.TurnToPprofLabel(&LabelSet{}).(pprof.LabelSet)
	assert.Equal(t, re1, pprof.LabelSet{})

	//test Label have something
	re2 := p.TurnToPprofLabel(&LabelSet{list: []label{
		{key: "minDurationThreshold", value: "0"},
		{key: "spanID", value: "0"},
		{key: "traceSegmentID", value: "test-segmentID"}}}).(pprof.LabelSet)
	l2 := pprof.Labels("minDurationThreshold", "0", "spanID", "0", "traceSegmentID", "test-segmentID")
	assert.Equal(t, re2, l2)
}
