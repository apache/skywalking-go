package reporter

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/pprof/profile"
	"github.com/pkg/errors"
	"runtime/pprof"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	"strconv"
	"sync"
	"time"
)

type TaskStatus int
type ProfileCtx struct {
	ctx       context.Context
	closeChan chan struct{}
}

const ChunkSize = 1024 * 1024
const (
	Pending TaskStatus = iota
	Running
	Finished
	Reported
)
const SegmentLabel = "traceSegmentID"
const SpanLabel = "spanID"

type Task struct {
	SerialNumber         string // uuid
	TaskId               string
	EndpointName         string // endpoint
	Duration             int    // monitoring duration (min)
	MinDurationThreshold int64  // starting monitoring time (ms)
	DumpPeriod           int    // monitoring interval (ms)
	MaxSamplingCount     int    // maximum number of samples
	StartTime            int64
	CreateTime           int64
	Status               TaskStatus // task execution status
	spanIds              []int32
	EndTime              int64 // task deadline
}

type currentTask struct {
	serialNumber         string // uuid
	taskId               string
	traceSegmentId       string
	spanIds              []int32 // spans exceeding MinDuration
	minDurationThreshold int64
	duration             int
}

type Result struct {
	Payload        [][]byte
	TraceSegmentID string
	TaskID         string
	SpanIDs        []int32
}

type ProfileManager struct {
	mu            sync.Mutex
	ctxs          map[string]ProfileCtx
	status        bool
	Tasks         map[string]*Task
	ReportResults chan Result
	buf           *bytes.Buffer // current profile buffer
	currentTask   *currentTask
}

func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		Tasks:         make(map[string]*Task),
		ReportResults: make(chan Result, 100),
		status:        false,
		ctxs:          make(map[string]ProfileCtx),
	}
}
func (m *ProfileManager) AddProfileTask(args []*common.KeyStringValuePair) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var task Task
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
	if _, exists := m.Tasks[task.TaskId]; exists {
		return
	}
	endTime := task.StartTime + int64(task.Duration)*60*1000
	task.EndTime = endTime
	task.Status = Pending
	m.Tasks[task.TaskId] = &task
}
func (m *ProfileManager) RemoveProfileTask() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, t := range m.Tasks {
		if t.Status == Reported || t.EndTime < time.Now().Unix() {
			delete(m.Tasks, k)
		}
	}
}
func (m *ProfileManager) getProfileTask(endpoint string) []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.Tasks {
		endTime := t.StartTime + int64(t.Duration)*60*1000
		if t.EndpointName == endpoint && t.StartTime <= time.Now().UnixMilli() && endTime > time.Now().UnixMilli() && t.Status == Pending {
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
func (m *ProfileManager) generateLabelCtx(traceSegmentID string) ProfileCtx {
	ctx := context.Background()
	labels := pprof.Labels(SegmentLabel, traceSegmentID)
	ctx = pprof.WithLabels(ctx, labels)
	closeChan := make(chan struct{}, 1)
	return ProfileCtx{
		ctx:       ctx,
		closeChan: closeChan,
	}
}
func (m *ProfileManager) generateCurrentTask(t *Task, traceSegmentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var c = currentTask{
		serialNumber:         t.SerialNumber,
		taskId:               t.TaskId,
		spanIds:              make([]int32, 0),
		traceSegmentId:       traceSegmentID,
		minDurationThreshold: t.MinDurationThreshold,
		duration:             t.Duration,
	}
	m.currentTask = &c
}
func (m *ProfileManager) ToProfile(endpoint string, traceSegmentID string) {
	//check if profiling
	if m.IfProfiling() {
		c := m.generateLabelCtx(traceSegmentID)
		m.ctxs[traceSegmentID] = c
		pprof.SetGoroutineLabels(c.ctx)
		return
	}

	tasks := m.getProfileTask(endpoint)
	if tasks != nil {
		err := m.StartProfiling(traceSegmentID)
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, v := range tasks {
			m.Tasks[v.TaskId].Status = Running
			//choose task to profiling
			task := v
			go func(task *Task) {
				m.generateCurrentTask(task, traceSegmentID)
				err = m.monitor()
				if err != nil {
					m.Tasks[task.TaskId].Status = Pending
					m.currentTask = nil
					m.status = false
					return
				}
				m.Tasks[task.TaskId].Status = Finished
				m.currentTask = nil
			}(task)
			break
		}
	}

}

func (m *ProfileManager) StartProfiling(traceSegmentID string) error {
	m.mu.Lock()
	m.status = true
	m.buf = &bytes.Buffer{}

	// Add main profiling context
	c := m.generateLabelCtx(traceSegmentID)
	pprof.SetGoroutineLabels(c.ctx)
	m.ctxs[traceSegmentID] = c
	m.mu.Unlock()

	if err := pprof.StartCPUProfile(m.buf); err != nil {
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
	case <-m.ctxs[m.currentTask.traceSegmentId].closeChan:
	}
	// Stop profiling
	pprof.StopCPUProfile()

	m.mu.Lock()
	// Store result
	data, err := m.GetResult()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	da, _, err := filterBySegmentAndSpanIDs(data, SegmentLabel, m.currentTask.traceSegmentId, SpanLabel, m.currentTask.spanIds)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	var re = Result{
		TaskID:         m.currentTask.taskId,
		TraceSegmentID: m.currentTask.traceSegmentId,
		SpanIDs:        m.currentTask.spanIds,
	}
	r := splitProfileData(da, ChunkSize)
	re.Payload = r
	m.ReportResults <- re

	// Update status
	delete(m.ctxs, m.currentTask.traceSegmentId)
	m.status = false
	m.mu.Unlock()
	return nil
}

func (m *ProfileManager) AddSpanId(segmentId string, spanID int32) {
	c, ok := m.ctxs[segmentId]
	if !ok || c.ctx == nil {
		return
	}
	spanCtx := pprof.WithLabels(c.ctx, pprof.Labels(SpanLabel, parseString(spanID)))
	pprof.SetGoroutineLabels(spanCtx)
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
	ctx, ok := m.ctxs[segmentID]
	if ok {
		select {
		case <-ctx.closeChan:
			fmt.Println("profile channel had already closed")
		default:
			close(ctx.closeChan)
			fmt.Println("profile channel closed")
		}
	}
	// Reset status
	m.status = false
}

func (m *ProfileManager) GetResult() ([]byte, error) {
	if m.buf == nil {
		return nil, errors.New("no buffer")
	}

	data := m.buf.Bytes()
	m.buf = nil
	return data, nil
}

func splitProfileData(data []byte, chunkSize int) [][]byte {
	var chunks [][]byte
	for len(data) > 0 {
		if len(data) < chunkSize {
			chunks = append(chunks, data)
			break
		}
		chunks = append(chunks, data[:chunkSize])
		data = data[chunkSize:]
	}
	return chunks
}

func (m *ProfileManager) CheckTimeIfEnough(traceSegmentId string, spanId int32, dur int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status && m.currentTask != nil {
		if m.currentTask.traceSegmentId != traceSegmentId {
			return
		}
		if dur > m.currentTask.minDurationThreshold {
			m.currentTask.spanIds = append(m.currentTask.spanIds, spanId)
		}
	}
}

// tools
func filterBySegmentAndSpanIDs(
	src []byte,
	segmentKey, segmentVal string,
	spanKey string,
	spanIDs []int32,
) ([]byte, *profile.Profile, error) {
	// Parse the profile from binary
	prof, err := profile.Parse(bytes.NewReader(src))
	if err != nil {
		return nil, nil, err
	}

	// Convert spanIDs to a string set for quick lookup
	spanIDSet := make(map[string]struct{}, len(spanIDs))
	for _, id := range spanIDs {
		spanIDSet[strconv.Itoa(int(id))] = struct{}{}
	}

	// Filter samples
	var filteredSamples []*profile.Sample
	for _, sample := range prof.Sample {
		// First match segmentId
		if segVals, ok := sample.Label[segmentKey]; ok {
			matchSeg := false
			for _, segVal := range segVals {
				if segVal == segmentVal {
					matchSeg = true
					break
				}
			}
			if !matchSeg {
				continue
			}
		} else {
			continue
		}

		// Then match spanId
		if spanVals, ok := sample.Label[spanKey]; ok {
			matchSpan := false
			for _, spanVal := range spanVals {
				if _, ok := spanIDSet[spanVal]; ok {
					matchSpan = true
					break
				}
			}
			if !matchSpan {
				continue
			}
		} else {
			continue
		}

		// Both conditions satisfied
		filteredSamples = append(filteredSamples, sample)
	}

	prof.Sample = filteredSamples

	// Write back to memory
	var buf bytes.Buffer
	if err = prof.Write(&buf); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), prof, nil
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
