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

package reporter

import (
	"time"

	common "github.com/apache/skywalking-go/protocols/collect/common/v3"
)

type ProfileTaskManager interface {
	// AddProfileTask add new profile task
	AddProfileTask(args []*common.KeyStringValuePair, t int64) int64
	GetProfileResults() chan ProfileResult
	ProfileFinish()
	RemoveProfileTask()
}

type TraceProfileTask struct {
	SerialNumber         string // uuid
	TaskID               string
	EndpointName         string // endpoint
	Duration             int    // monitoring duration (min)
	MinDurationThreshold int64  // starting monitoring time (ms)
	DumpPeriod           int    // monitoring interval (ms)
	MaxSamplingCount     int    // maximum number of samples
	StartTime            time.Time
	CreateTime           time.Time
	Status               ProfileTaskStatus // task execution status
	EndTime              time.Time         // task deadline
}

type ProfileResult struct {
	Payload        []byte
	TraceSegmentID string
	TaskID         string
	IsLast         bool
}
