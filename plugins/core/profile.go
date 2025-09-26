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
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	common "github.com/apache/skywalking-go/protocols/collect/common/v3"
)

type profileLabels struct {
	labels *LabelSet
}

const (
	maxSendQueueSize int32         = 100
	timeOut          time.Duration = 2 * time.Minute
	ChunkSize                      = 1024 * 1024
	TraceLabel                     = "traceID"
	SegmentLabel                   = "traceSegmentID"
	MinDurationLabel               = "minDurationThreshold"
	SpanLabel                      = "spanID"
)

type currentTask struct {
	serialNumber         string // uuid
	taskID               string
	minDurationThreshold int64
	endpointName         string
	endTime              time.Time
	duration             int
}

type ProfileManager struct {
	mu                 sync.Mutex
	TraceProfileTask   *reporter.TraceProfileTask
	rawCh              chan profileRawData
	FinalReportResults chan reporter.ProfileResult
	profilingWriter    *ProfilingWriter
	profileEvents      *TraceProfilingEventManager
	currentTask        *currentTask
	Log                operator.LogOperator
	counter            atomic.Int32
}

func (m *ProfileManager) initReportChannel() {
	// Original channel for receiving raw data chunks sent by the Writer
	rawCh := make(chan profileRawData, maxSendQueueSize)
	m.rawCh = rawCh
	var d []byte
	// Start a goroutine to supplement each data chunk with business information
	go func() {
		for rawResult := range rawCh {
			d = append(d, rawResult.data...)
			m.mu.Lock()
			// Get business information from currentTask
			if m.currentTask == nil {
				m.Log.Info("no task")
				m.mu.Unlock()
				continue // Task has ended, ignore
			}
			task := m.currentTask
			m.mu.Unlock()

			if rawResult.isLast {
				m.FinalReportResults <- reporter.ProfileResult{
					TaskID:  task.taskID,
					Payload: rawResult.data,
					IsLast:  rawResult.isLast,
				}
				m.mu.Lock()
				if m.TraceProfileTask == nil {
					m.Log.Warn("no TraceProfileTask before finish profile")
				}
				m.TraceProfileTask.Status = reporter.Finished
				m.currentTask = nil
				m.profileEvents.BaseEventStatus[CurTaskExist] = false
				m.mu.Unlock()
			} else {
				m.FinalReportResults <- reporter.ProfileResult{
					TaskID:  task.taskID,
					Payload: rawResult.data,
					IsLast:  rawResult.isLast,
				}
			}
		}
	}()
}

func NewProfileManager(log operator.LogOperator) *ProfileManager {
	pm := &ProfileManager{
		FinalReportResults: make(chan reporter.ProfileResult, maxSendQueueSize),
		profileEvents:      NewEventManager(),
	}
	pm.RegisterProfileEvents()

	if log == nil {
		log = newDefaultLogger()
	}
	pm.Log = log
	pm.initReportChannel()
	pm.profilingWriter = NewProfilingWriter(
		ChunkSize,
		pm.rawCh,
	)
	return pm
}

func (m *ProfileManager) AddProfileTask(args []*common.KeyStringValuePair) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var task reporter.TraceProfileTask
	for _, arg := range args {
		switch arg.Key {
		case "TaskId":
			task.TaskID = arg.Value
		case "EndpointName":
			task.EndpointName = arg.Value
		case "Duration":
			// Duration min
			task.Duration = parseInt(arg.Value)
		case "MinDurationThreshold":
			task.MinDurationThreshold = parseInt64(arg.Value)
		case "DumpPeriod":
			task.DumpPeriod = parseInt(arg.Value)
		case "MaxSamplingCount":
			task.MaxSamplingCount = parseInt(arg.Value)
		case "StartTime":
			task.StartTime = time.UnixMilli(parseInt64(arg.Value))
		case "CreateTime":
			task.CreateTime = time.UnixMilli(parseInt64(arg.Value))
		case "SerialNumber":
			task.SerialNumber = arg.Value
		}
	}
	m.Log.Info("adding profile task:", task)
	if m.TraceProfileTask != nil {
		return
	}
	endTime := task.StartTime.Add(time.Duration(task.Duration) * time.Minute)
	task.EndTime = endTime
	task.Status = reporter.Pending
	m.TraceProfileTask = &task
	m.TrySetCurrentTaskAndStartProfile(&task)
}

func (m *ProfileManager) RemoveProfileTask() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.TraceProfileTask == nil {
		return
	}
	if m.TraceProfileTask.Status == reporter.Reported ||
		time.Now().After(m.TraceProfileTask.EndTime.Add(timeOut)) {
		m.TraceProfileTask = nil
	}
}

func (m *ProfileManager) tryStartCPUProfiling() {
	ok, err := m.profileEvents.ExecuteComplexEvent(CouldProfile)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
		return
	}
	t := m.TraceProfileTask
	if ok && t.Status == reporter.Pending {
		err := pprof.StartCPUProfile(m.profilingWriter)
		if err != nil {
			m.Log.Info("failed to start cpu profiling", err)
			return
		}
		err = m.profileEvents.UpdateBaseEventStatus(IfProfiling, true)
		if err != nil {
			m.Log.Errorf("update profile event error:%v", err)
		}
		t.Status = reporter.Running
		go m.monitor()
	}
}

func (m *ProfileManager) CheckIfProfileTarget(endpoint string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentTask == nil {
		return false
	}
	return m.currentTask.endpointName == endpoint
}

func (m *ProfileManager) IfProfiling() bool {
	ok, err := m.profileEvents.GetBaseEventStatus(IfProfiling)
	if err != nil {
		m.Log.Errorf("get profile event error:%v", err)
		return false
	}
	return ok
}

func (m *ProfileManager) TrySetCurrentTaskAndStartProfile(task *reporter.TraceProfileTask) {
	if m.currentTask != nil && time.Now().Before(m.currentTask.endTime.Add(timeOut)) {
		return
	}
	ok, err := m.profileEvents.ExecuteComplexEvent(CouldSetCurTask)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
	}
	if ok {
		m.generateCurrentTask(task)
		m.tryStartCPUProfiling()
	}
}

func (m *ProfileManager) generateProfileLabels(traceSegmentID string, minDurationThreshold int64) profileLabels {
	var l = LabelSet{}
	l = UpdateTraceLabels(l, SegmentLabel, traceSegmentID, MinDurationLabel, strconv.FormatInt(minDurationThreshold, 10))
	return profileLabels{
		labels: &l,
	}
}

func (m *ProfileManager) generateCurrentTask(t *reporter.TraceProfileTask) {
	var c = currentTask{
		serialNumber:         t.SerialNumber,
		taskID:               t.TaskID,
		minDurationThreshold: t.MinDurationThreshold,
		duration:             t.Duration,
		endpointName:         t.EndpointName,
		endTime:              t.EndTime,
	}
	m.currentTask = &c
	err := m.profileEvents.UpdateBaseEventStatus(CurTaskExist, true)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
	}
}

func (m *ProfileManager) TryToAddSegmentLabelSet(traceSegmentID string) {
	if m.currentTask != nil {
		c := m.generateProfileLabels(traceSegmentID, m.currentTask.minDurationThreshold)
		SetGoroutineLabels(c.labels)
		return
	}
}

func (m *ProfileManager) monitor() {
	<-time.After(time.Duration(m.currentTask.duration) * time.Minute)
	pprof.StopCPUProfile()
	err := m.profileEvents.UpdateBaseEventStatus(IfProfiling, false)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
	}
	if m.profilingWriter != nil {
		m.profilingWriter.Flush()
	}
}

func (m *ProfileManager) AddSpanID(traceID, segmentID string, spanID int32) {
	l := m.AddSkyLabels(traceID, segmentID, spanID).(*LabelSet)
	SetGoroutineLabels(l)
}

func (m *ProfileManager) IncCounter() {
	m.counter.Add(1)
	err := m.profileEvents.UpdateBaseEventStatus(HasWorthRequeue, true)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
	}
}

func (m *ProfileManager) DecCounter() {
	m.mu.Lock()
	ct := m.counter.Add(-1)
	if ct == 0 {
		m.mu.Unlock()
		err := m.profileEvents.UpdateBaseEventStatus(HasWorthRequeue, false)
		if err != nil {
			m.Log.Errorf("profile event error:%v", err)
		}
		return
	}
	m.mu.Unlock()
}

func (m *ProfileManager) GetProfileResults() chan reporter.ProfileResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.FinalReportResults
}

func (m *ProfileManager) ProfileFinish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TraceProfileTask.Status = reporter.Reported
}

func parseInt64(value string) int64 {
	v, _ := strconv.ParseInt(value, 10, 64)
	return v
}

func parseInt(value string) int {
	v, _ := strconv.Atoi(value)
	return v
}
func parseString(value int32) string {
	str := strconv.Itoa(int(value))
	return str
}
