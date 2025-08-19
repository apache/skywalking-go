package reporter

import common "skywalking.apache.org/repo/goapi/collect/common/v3"

type ProfileTaskManager interface {
	// AddProfileTask add new profile task
	AddProfileTask(args []*common.KeyStringValuePair)
	GetProfileResults() chan ProfileResult
	ProfileFinish(taskId string)
	RemoveProfileTask()
}

type ProfileTask struct {
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

type ProfileResult struct {
	Payload        [][]byte
	TraceSegmentID string
	TaskID         string
	SpanIDs        []int32
}
