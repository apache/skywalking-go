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
	SerialNumber         string //uuid
	TaskId               string
	EndpointName         string //端点
	Duration             int    //监控持续时间(min)
	MinDurationThreshold int64  //起始监控时间(ms)
	DumpPeriod           int    //监控间隔(ms)
	MaxSamplingCount     int    //最大采样数
	StartTime            int64
	CreateTime           int64
	Status               TaskStatus //任务执行状态
	spanIds              []int32
	EndTime              int64 //任务deadline
}
type currentTask struct {
	serialNumber         string //uuid
	taskId               string
	traceSegmentId       string
	spanIds              []int32 //超过MinDuration的span
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
	buf           *bytes.Buffer // 当前 profile 的 buffer
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
			// Duration 单位为分钟
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
	fmt.Println("adding task:", task)
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
	//检测当下是否正在profiling
	if m.IfProfiling() {
		c := m.generateLabelCtx(traceSegmentID)
		m.ctxs[traceSegmentID] = c
		pprof.SetGoroutineLabels(c.ctx)
		return
	}
	t := time.Now().UnixMilli()
	tasks := m.GetProfileTask(endpoint)
	if tasks != nil {
		err := m.StartProfiling(traceSegmentID)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, v := range tasks {
			//删除过期任务
			if v.EndTime < t {
				m.mu.Lock()
				delete(m.Tasks, v.TaskId)
				m.mu.Unlock()
				continue
			}
			m.Tasks[v.TaskId].Status = Running
			//执行profiling
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

	//添加profiling主上下文
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
	// 超时结束
	case <-time.After(time.Duration(m.currentTask.duration) * time.Minute):

	case <-m.ctxs[m.currentTask.traceSegmentId].closeChan: // 手动结束
	}
	// 停止
	pprof.StopCPUProfile()
	m.mu.Lock()
	//存储结果
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
	//修改状态
	delete(m.ctxs, m.currentTask.traceSegmentId)
	m.status = false
	m.mu.Unlock()
	return nil
}
func (m *ProfileManager) AddSpanId(segmentId string, spanID int32) {
	fmt.Println("adding span:", segmentId)
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

	// 先判断是否在profiling，且当前任务存在
	if !m.status || m.currentTask == nil {
		return
	}

	//  校验当前任务的traceSegmentId是否匹配
	if m.currentTask.traceSegmentId != segmentID {
		return
	}

	// 安全关闭通道（确保存在）
	ctx, ok := m.ctxs[segmentID]
	if ok {
		select {
		case <-ctx.closeChan:
			fmt.Println("profile chan had closed")
		default:
			close(ctx.closeChan)
			fmt.Println("profile chan closed")
		}
	}
	// 重置状态
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
	// 从二进制解析 profile
	prof, err := profile.Parse(bytes.NewReader(src))
	if err != nil {
		return nil, nil, err
	}

	// 将 spanIDs 转为 string 集合方便快速查找
	spanIDSet := make(map[string]struct{}, len(spanIDs))
	for _, id := range spanIDs {
		spanIDSet[strconv.Itoa(int(id))] = struct{}{}
	}

	// 过滤样本
	var filteredSamples []*profile.Sample
	for _, sample := range prof.Sample {
		// 先匹配 segmentId
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

		// 再匹配 spanId
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

		// 两个条件都满足
		filteredSamples = append(filteredSamples, sample)
	}

	prof.Sample = filteredSamples

	// 写回内存
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
