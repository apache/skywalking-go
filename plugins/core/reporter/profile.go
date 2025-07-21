package reporter

import (
	"sync"
	//profilev3 "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
)

type ProfileTaskStatus int

const (
	Pending ProfileTaskStatus = iota
	Running
	Finished
	Reported
)

type ProfileTask struct {
	SerialNumber         string //uuid
	TaskId               string
	EndpointName         string //端点
	Duration             int    //监控持续时间(min)
	MinDurationThreshold int    //起始监控时间(ms)
	DumpPeriod           int    //监控间隔(ms)
	MaxSamplingCount     int    //最大采样数
	StartTime            uint64
	CreateTime           uint64
	Status               ProfileTaskStatus //任务执行状态
}
type ProfileManager struct {
	mu    sync.Mutex
	Tasks map[string]*ProfileTask
}

func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		Tasks: make(map[string]*ProfileTask),
	}
}
func (m *ProfileManager) AddProfileTask(task *ProfileTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Tasks[task.SerialNumber] = task
}
