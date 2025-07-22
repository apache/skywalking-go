package reporter

import (
	"fmt"
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
	mu    sync.Mutex
	Tasks map[string]*Task
}

func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		Tasks: make(map[string]*Task),
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
	fmt.Println(task.EndpointName)
	m.Tasks[task.SerialNumber] = &task
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
func (m *ProfileManager) Display(endpoint string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result string
	for _, t := range m.Tasks {
		if t.EndpointName == endpoint {
			result += t.SerialNumber
		}
	}
	return result
}
func parseInt64(value string) int64 {
	v, _ := strconv.ParseInt(value, 10, 64)
	return v
}

func parseInt(value string) int {
	v, _ := strconv.Atoi(value)
	return v
}
