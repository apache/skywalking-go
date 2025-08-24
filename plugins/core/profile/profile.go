package profile

import (
	"fmt"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"runtime/pprof"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	"strconv"
	"sync"
	"time"
)

type profileLabels struct {
	labels    *LabelSet
	closeChan chan struct{}
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
	traceSegmentId       string
	minDurationThreshold int64
	duration             int
}

type ProfileManager struct {
	mu                 sync.Mutex
	labelSets          map[string]profileLabels
	status             bool
	TraceProfileTasks  map[string]*reporter.TraceProfileTask
	rawCh              chan profileRawData
	FinalReportResults chan reporter.ProfileResult
	profilingWriter    *ProfilingWriter
	currentTask        *currentTask
}

func (m *ProfileManager) initReportChannel() {
	// Original channel for receiving raw data chunks sent by the Writer
	rawCh := make(chan profileRawData, maxSendQueueSize)
	m.rawCh = rawCh

	// Start a goroutine to supplement each data chunk with business information
	go func() {
		for rawResult := range rawCh {
			m.mu.Lock()
			// Get business information from currentTask
			task := m.currentTask
			m.mu.Unlock()

			if task == nil {
				fmt.Println("no task")
				continue // Task has ended, ignore
			}

			if rawResult.isLast {
				m.FinalReportResults <- reporter.ProfileResult{
					TaskID:         task.taskId,
					TraceSegmentID: task.traceSegmentId,
					Payload:        rawResult.data,
					IsLast:         rawResult.isLast,
				}
			} else {
				m.FinalReportResults <- reporter.ProfileResult{
					TaskID:         task.taskId,
					TraceSegmentID: task.traceSegmentId,
					Payload:        rawResult.data,
					IsLast:         rawResult.isLast,
				}
			}

		}
	}()
}

func NewProfileManager() *ProfileManager {
	pm := &ProfileManager{
		TraceProfileTasks:  make(map[string]*reporter.TraceProfileTask),
		FinalReportResults: make(chan reporter.ProfileResult, maxSendQueueSize),
		status:             false,
		labelSets:          make(map[string]profileLabels),
	}
	pm.initReportChannel()
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
	fmt.Println("adding profile task:", task)
	if _, exists := m.TraceProfileTasks[task.TaskId]; exists {
		return
	}
	endTime := task.StartTime + int64(task.Duration)*60*1000
	task.EndTime = endTime
	task.Status = reporter.Pending
	m.TraceProfileTasks[task.TaskId] = &task
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

func (m *ProfileManager) getProfileTask(endpoint string) []*reporter.TraceProfileTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*reporter.TraceProfileTask
	for _, t := range m.TraceProfileTasks {
		endTime := t.StartTime + int64(t.Duration)*60*1000
		if t.EndpointName == endpoint && t.StartTime <= time.Now().UnixMilli() && endTime > time.Now().UnixMilli() && t.Status == reporter.Pending {
			result = append(result, t)
		}
	}
	return result
}

func (m *ProfileManager) IfProfiling() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *ProfileManager) generateProfileLabels(traceSegmentID string, minDurationThreshold int64) profileLabels {
	var l = &LabelSet{}
	if minDurationThreshold == 0 {
		l = Labels(l, SegmentLabel, traceSegmentID)
	} else {
		l = Labels(l, SegmentLabel, traceSegmentID, MinDurationLabel, strconv.FormatInt(minDurationThreshold, 10))
	}
	closeChan := make(chan struct{}, 1)
	return profileLabels{
		labels:    l,
		closeChan: closeChan,
	}
}

func (m *ProfileManager) generateCurrentTask(t *reporter.TraceProfileTask, traceSegmentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var c = currentTask{
		serialNumber:         t.SerialNumber,
		taskId:               t.TaskId,
		traceSegmentId:       traceSegmentID,
		minDurationThreshold: t.MinDurationThreshold,
		duration:             t.Duration,
	}
	m.currentTask = &c
}

func (m *ProfileManager) ToProfile(endpoint string, traceSegmentID string) {
	//check if profiling
	if m.IfProfiling() {
		c := m.generateProfileLabels(traceSegmentID, 0)
		m.labelSets[traceSegmentID] = c
		SetGoroutineLabels(c.labels)
		return
	}

	tasks := m.getProfileTask(endpoint)
	if tasks != nil {
		for _, v := range tasks {
			m.TraceProfileTasks[v.TaskId].Status = reporter.Running
			//choose task to profiling
			task := v
			m.generateCurrentTask(task, traceSegmentID)
			err := m.StartProfiling(traceSegmentID)
			if err != nil {
				fmt.Println(err)
				return
			}
			go func(task *reporter.TraceProfileTask) {
				err = m.monitor()
				if err != nil {
					m.TraceProfileTasks[task.TaskId].Status = reporter.Pending
					m.currentTask = nil
					m.status = false
					return
				}
				m.TraceProfileTasks[task.TaskId].Status = reporter.Finished
			}(task)
			break
		}
	}

}

func (m *ProfileManager) StartProfiling(traceSegmentID string) error {
	m.mu.Lock()
	m.status = true
	m.profilingWriter = NewProfilingWriter(
		ChunkSize,
		m.rawCh,
	)
	// Add main profiling context
	c := m.generateProfileLabels(traceSegmentID, m.currentTask.minDurationThreshold)
	SetGoroutineLabels(c.labels)
	m.labelSets[traceSegmentID] = c
	m.mu.Unlock()

	if err := pprof.StartCPUProfile(m.profilingWriter); err != nil {
		m.mu.Lock()
		m.status = false
		m.mu.Unlock()
		return err
	}
	return nil
}

func (m *ProfileManager) monitor() error {
	select {
	// End on timeout
	case <-time.After(time.Duration(m.currentTask.duration) * time.Minute):

	// End manually
	case <-m.labelSets[m.currentTask.traceSegmentId].closeChan:
	}
	// Stop profiling
	pprof.StopCPUProfile()
	m.profilingWriter.Flush()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labelSets = make(map[string]profileLabels)
	return nil
}

func (m *ProfileManager) AddSpanId(segmentId string, spanID int32) {
	c, ok := m.labelSets[segmentId]
	if !ok || c.labels == nil {
		return
	}
	nowLabels := GetPprofLabelSet()
	afterAdd := Labels(nowLabels, SpanLabel, parseString(spanID))
	SetGoroutineLabels(afterAdd)
}

func (m *ProfileManager) EndProfiling(segmentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if profiling is ongoing and current task exists
	if !m.status || m.currentTask == nil {
		return
	}

	// Verify if the current task's traceSegmentId matches
	if m.currentTask.traceSegmentId != segmentID {
		return
	}

	// Safely close the channel (ensure it exists)
	ctx, ok := m.labelSets[segmentID]
	if ok {
		select {
		case <-ctx.closeChan:
			fmt.Println("profile channel had already closed")
		default:
			close(ctx.closeChan)
			fmt.Println("profile channel closed")
		}
	}
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
	m.currentTask = nil
	m.status = false
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
