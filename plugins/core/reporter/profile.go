package reporter

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"runtime/pprof"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	"strconv"
	"sync"
	"time"
)

type TaskStatus int

const (
	Pending TaskStatus = iota
	Running
	Finished
	Reported
)

type Task struct {
	SerialNumber         string //uuid
	TaskId               string
	EndpointName         string //端点
	Duration             int    //监控持续时间(min)
	MinDurationThreshold int    //起始监控时间(ms)
	DumpPeriod           int    //监控间隔(ms)
	MaxSamplingCount     int    //最大采样数
	StartTime            int64
	CreateTime           int64
	Status               TaskStatus //任务执行状态
}
type ProfileManager struct {
	mu       sync.Mutex
	ctx      context.Context
	status   bool
	Tasks    map[string]*Task
	buf      *bytes.Buffer // 当前 profile 的 buffer
	stopChan chan struct{} //结束信号
}

func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		Tasks:  make(map[string]*Task),
		status: false,
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
			// Duration 单位为分钟
			task.Duration = parseInt(arg.Value)
		case "MinDurationThreshold":
			task.MinDurationThreshold = parseInt(arg.Value)
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
	task.Status = Pending
	fmt.Println("adding task:", task)
	m.Tasks[task.TaskId] = &task
}
func (m *ProfileManager) RemoveProfileTask() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, t := range m.Tasks {
		if t.Status == Reported {
			delete(m.Tasks, k)
		}
	}
}
func (m *ProfileManager) GetProfileTask(endpoint string) []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.Tasks {
		if t.EndpointName == endpoint && t.StartTime <= time.Now().UnixMilli() && t.Status == Pending {
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

func (m *ProfileManager) ToProfile(endpoint string, traceSegmentID string) {
	t := time.Now().UnixMilli()

	for _, v := range m.GetProfileTask(endpoint) {
		//删除过期任务
		if v.StartTime+int64(v.Duration)*int64(time.Minute) < t {
			delete(m.Tasks, v.SerialNumber)
			continue
		}
		m.Tasks[v.TaskId].Status = Running
		//执行profiling
		task := v
		go func(task *Task) {
			err := m.StartProfiling(task, traceSegmentID)
			if err != nil {
				m.Tasks[task.TaskId].Status = Pending
				return
			}
			m.Tasks[task.TaskId].Status = Finished
		}(task)

	}
}
func (m *ProfileManager) StartProfiling(t *Task, traceSegmentID string) error {
	m.mu.Lock()
	if m.status {
		m.mu.Unlock()
		return errors.New("profile is already running")
	}
	m.status = true
	m.buf = &bytes.Buffer{}
	m.stopChan = make(chan struct{})
	m.mu.Unlock()
	//添加profiling主上下文
	ctx := context.Background()
	labels := pprof.Labels("traceSegmentID", traceSegmentID)
	ctx = pprof.WithLabels(ctx, labels)
	pprof.SetGoroutineLabels(ctx)
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()

	if err := pprof.StartCPUProfile(m.buf); err != nil {
		m.mu.Lock()
		m.status = false
		m.mu.Unlock()
		return err
	}

	select {
	// 超时结束
	case <-time.After(time.Duration(t.Duration) * time.Minute):

	case <-m.stopChan: // 手动结束
	}
	// 停止
	pprof.StopCPUProfile()
	m.mu.Lock()
	m.status = false
	close(m.stopChan)
	m.stopChan = nil
	m.mu.Unlock()
	return nil
}
func (m *ProfileManager) AddSpanId(spanID string) {
	if m.ctx == nil {
		return
	}

	spanCtx := pprof.WithLabels(m.ctx, pprof.Labels("spanID", spanID))
	pprof.SetGoroutineLabels(spanCtx)
}
func (m *ProfileManager) EndProfiling() {
	m.mu.Lock()
	defer m.mu.Unlock()
	//如果是false,即没启动，就直接返回
	if !m.status {
		return
	}

	if m.stopChan != nil {
		// 通知 goroutine 停止
		close(m.stopChan)
	}
}

func (m *ProfileManager) ReportResult() ([]byte, error) {
	// goroutine 会调用 StopCPUProfile 并清理状态
	// 但 StopCPUProfile 不是完全同步的，最好等待 status=false
	for m.status {
		m.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		m.mu.Lock()
	}
	if m.buf == nil {
		return nil, errors.New("no buffer")
	}

	data := m.buf.Bytes()
	m.buf = nil
	m.ctx = nil
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
func parseInt64(value string) int64 {
	v, _ := strconv.ParseInt(value, 10, 64)
	return v
}

func parseInt(value string) int {
	v, _ := strconv.Atoi(value)
	return v
}
