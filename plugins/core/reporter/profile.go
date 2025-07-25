package reporter

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"runtime/pprof"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	"strconv"
	"sync"
	"time"
)

type TaskStatus int

var cpuProfileLock sync.Mutex

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
	fmt.Println("adding task:", task)
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
func (m *ProfileManager) ToProfile(endpoint string, traceId string) {
	t := time.Now().UnixMilli()
	//fmt.Println(traceId + "  " + endpoint + "  " + "start")
	for _, v := range m.GetProfileTask(endpoint) {
		//删除过期任务
		if v.StartTime+int64(v.Duration)*int64(time.Minute) < t {
			delete(m.Tasks, v.SerialNumber)
			continue
		}
		v.Status = Running
		//执行profiling
		task := v
		go func(task *Task) {
			data, err := StartProfiling(task)
			if err != nil {
				task.Status = Pending
				return
			}
			fmt.Println(data)
		}(task)

	}
}
func StartProfiling(t *Task) ([]byte, error) {
	var buf bytes.Buffer
	if cpuProfileLock.TryLock() {
		defer cpuProfileLock.Unlock()
		// 开始写入 profiling 数据到内存
		if err := pprof.StartCPUProfile(&buf); err != nil {
			fmt.Printf("could not start CPU profile: %v\n", err)
			return nil, err
		}

		// 模拟运行（持续 t.Duration min）
		time.Sleep(time.Duration(t.Duration) * time.Minute)
		pprof.StopCPUProfile()
		return buf.Bytes(), nil
		// profiling 逻辑
	} else {
		//fmt.Println("profile is already running, skip this task")
		return nil, errors.New("profile is already running")
	}

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
