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
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLabels(t *testing.T) {
	var ctx = context.Background()
	labels := pprof.Labels("test1", "test1_label", "test2", "test2_label")
	ctx = pprof.WithLabels(ctx, labels)
	pprof.SetGoroutineLabels(ctx)
	p := NewProfileManager()
	ls := p.GetPprofLabelSet().(*LabelSet)
	ts := LabelSet{list: []label{{"test1", "test1_label"}, {"test2", "test2_label"}}}
	assert.Equal(t, ts, *ls)
}

func TestSetLabels(t *testing.T) {
	ts := &LabelSet{list: []label{{"test1", "test1_label"}, {"test2", "test2_label"}}}
	labels := Labels(ts, "test3", "test3_label")
	SetGoroutineLabels(labels)
	p := NewProfileManager()
	re := p.GetPprofLabelSet().(*LabelSet)
	assert.Equal(t, re, ts)
}
