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

const (
	IfProfiling  TraceProfilingBaseEvent = "IfProfiling"
	CurTaskExist TraceProfilingBaseEvent = "CurTaskExist"

	CouldProfile    TraceProfilingComplexEvent = "CouldProfile"
	CouldSetCurTask TraceProfilingComplexEvent = "CouldSetCurTask"
)

func (m *ProfileManager) RegisterProfileEvents() {
	m.profileEvents.RegisterBaseEvent(IfProfiling, false)
	m.profileEvents.RegisterBaseEvent(CurTaskExist, false)
	var r1 = TraceProfilingRule{
		Event: IfProfiling,
		Op:    OpNothing,
		IsNot: true,
	}
	var r2 = TraceProfilingRule{
		Event: CurTaskExist,
		Op:    OpAnd,
		IsNot: false,
	}
	var r3 = TraceProfilingRule{
		Event: CurTaskExist,
		Op:    OpAnd,
		IsNot: true,
	}
	m.profileEvents.RegisterComplexEvent(CouldProfile, &TraceProfilingExprNode{
		Rules: []TraceProfilingRule{
			r1, r2,
		},
		Event: CouldProfile,
	})
	m.profileEvents.RegisterComplexEvent(CouldSetCurTask, &TraceProfilingExprNode{
		Rules: []TraceProfilingRule{
			r1, r3,
		},
		Event: CouldSetCurTask,
	})
}
