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
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	common "github.com/apache/skywalking-go/protocols/collect/common/v3"
	"os"
	"runtime/pprof"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type profileLabels struct {
	labels *LabelSet
}

const (
	maxSendQueueSize int32 = 100
	ChunkSize              = 1024 * 1024
	SegmentLabel           = "traceSegmentID"
	MinDurationLabel       = "minDurationThreshold"
	SpanLabel              = "spanID"
)

type currentTask struct {
	serialNumber         string // uuid
	taskId               string
	minDurationThreshold int64
	endpointName         string
	duration             int
}

type ProfileManager struct {
	mu                 sync.Mutex
	labelSets          map[string]profileLabels
	TraceProfileTasks  map[string]*reporter.TraceProfileTask
	rawCh              chan profileRawData
	FinalReportResults chan reporter.ProfileResult
	profilingWriter    *ProfilingWriter
	profileEvents      *EventManager
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
			task := m.currentTask
			m.mu.Unlock()

			if task == nil {
				m.Log.Info("no task\n")
				continue // Task has ended, ignore
			}

			if rawResult.isLast {
				m.FinalReportResults <- reporter.ProfileResult{
					TaskID:  task.taskId,
					Payload: rawResult.data,
					IsLast:  rawResult.isLast,
				}
				m.mu.Lock()
				m.TraceProfileTasks[m.currentTask.taskId].Status = reporter.Finished
				m.currentTask = nil
				m.profileEvents.BaseEventStatus[CurTaskExist] = false
				m.mu.Unlock()
				f, _ := os.Create("cpu.pprof.gz")
				f.Write(d)
				f.Close()
			} else {
				m.FinalReportResults <- reporter.ProfileResult{
					TaskID:  task.taskId,
					Payload: rawResult.data,
					IsLast:  rawResult.isLast,
				}
			}

		}
	}()
}

func NewProfileManager(log operator.LogOperator) *ProfileManager {
	pm := &ProfileManager{
		TraceProfileTasks:  make(map[string]*reporter.TraceProfileTask),
		FinalReportResults: make(chan reporter.ProfileResult, maxSendQueueSize),
		labelSets:          make(map[string]profileLabels),
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
			task.TaskId = arg.Value
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
			task.StartTime = parseInt64(arg.Value)
		case "CreateTime":
			task.CreateTime = parseInt64(arg.Value)
		case "SerialNumber":
			task.SerialNumber = arg.Value
		}
	}
	m.Log.Info("adding profile task:", task, "\n")
	if _, exists := m.TraceProfileTasks[task.TaskId]; exists {
		return
	}
	endTime := task.StartTime + int64(task.Duration)*60*1000
	task.EndTime = endTime
	task.Status = reporter.Pending
	m.TraceProfileTasks[task.TaskId] = &task
	m.TrySetCurrentTask(&task)
	m.tryStartCpuProfiling()
}

func (m *ProfileManager) RemoveProfileTask() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, t := range m.TraceProfileTasks {
		if t.Status == reporter.Reported || t.EndTime < time.Now().Unix() {
			delete(m.TraceProfileTasks, k)
		}
	}
}

func (m *ProfileManager) tryStartCpuProfiling() {
	ok, err := m.profileEvents.ExecuteComplexEvent(CouldProfile)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
		return
	}
	t := m.TraceProfileTasks[m.currentTask.taskId]
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
	return m.profileEvents.BaseEventStatus[IfProfiling]
}

func (m *ProfileManager) TrySetCurrentTask(task *reporter.TraceProfileTask) {
	ok, err := m.profileEvents.ExecuteComplexEvent(CouldSetCurTask)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
	}
	if ok {
		m.generateCurrentTask(task)
	}
}

func (m *ProfileManager) generateProfileLabels(traceSegmentID string, minDurationThreshold int64) profileLabels {
	var l = &LabelSet{}

	l = Labels(l, SegmentLabel, traceSegmentID, MinDurationLabel, strconv.FormatInt(minDurationThreshold, 10))

	return profileLabels{
		labels: l,
	}
}

func (m *ProfileManager) generateCurrentTask(t *reporter.TraceProfileTask) {
	var c = currentTask{
		serialNumber:         t.SerialNumber,
		taskId:               t.TaskId,
		minDurationThreshold: t.MinDurationThreshold,
		duration:             t.Duration,
		endpointName:         t.EndpointName,
	}
	m.currentTask = &c
	err := m.profileEvents.UpdateBaseEventStatus(CurTaskExist, true)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
	}
}

func (m *ProfileManager) TryToAddSegmentID(traceSegmentID string) {
	if m.profileEvents.BaseEventStatus[IfProfiling] && m.currentTask != nil {
		c := m.generateProfileLabels(traceSegmentID, m.currentTask.minDurationThreshold)
		m.labelSets[traceSegmentID] = c
		SetGoroutineLabels(c.labels)
		return
	}

}

func (m *ProfileManager) monitor() {
	done := make(chan struct{})
	var zeroSince time.Time
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			currentCounter := m.counter.Load()
			if currentCounter == 0 {
				if zeroSince.IsZero() {
					zeroSince = time.Now()
				}

				if time.Since(zeroSince) >= 30*time.Second {
					close(done)
					return
				}
			} else {
				zeroSince = time.Time{}
			}
		}
	}()

	select {
	case <-time.After(time.Duration(m.currentTask.duration) * time.Minute):

	case <-done:
	}
	pprof.StopCPUProfile()
	if m.profilingWriter != nil {
		m.profilingWriter.Flush()
	}
}

func (m *ProfileManager) AddSpanId(segmentId string, spanID int32) {
	c, ok := m.labelSets[segmentId]
	if !ok || c.labels == nil {
		return
	}
	nowLabels := m.GetPprofLabelSet().(*LabelSet)
	afterAdd := Labels(nowLabels, SpanLabel, parseString(spanID))
	SetGoroutineLabels(afterAdd)
}

func (m *ProfileManager) IncCounter() {
	m.counter.Add(1)
	err := m.profileEvents.UpdateBaseEventStatus(HasWorthRequeue, true)
	if err != nil {
		m.Log.Errorf("profile event error:%v", err)
	}
}

func (m *ProfileManager) DecCounter(segmentId string) {
	m.mu.Lock()
	ct := m.counter.Add(-1)
	delete(m.labelSets, segmentId)
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

func (m *ProfileManager) ProfileFinish(taskId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TraceProfileTasks[taskId].Status = reporter.Reported
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
