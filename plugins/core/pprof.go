package core

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync/atomic"
	"time"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
)

// CPU profiling state to ensure only one CPU profiling task runs at a time
var isRunning atomic.Bool

func init() {
	reporter.NewPprofTaskCommand = NewPprofTaskCommand
}

type PprofTaskCommandImpl struct {
	serialNumber string
	// Pprof Task ID
	taskId string
	// Type of profiling (CPU/Heap/Block/Mutex)
	events string
	// unit is minute
	duration time.Duration
	// Unix timestamp in milliseconds when the task was created
	createTime int64
	// Processed dump period (unit is hz for CPU, converted for internal use)
	dumpPeriod int

	// for pprof task service
	pprofFilePath string
	logger        operator.LogOperator
	manager       reporter.PprofReporter
}

func NewPprofTaskCommand(serialNumber, taskId, events string, duration time.Duration, createTime int64, dumpPeriod int, pprofFilePath string, logger operator.LogOperator, manager reporter.PprofReporter) reporter.PprofTaskCommand {
	return &PprofTaskCommandImpl{
		serialNumber:  serialNumber,
		taskId:        taskId,
		events:        events,
		duration:      duration,
		createTime:    createTime,
		dumpPeriod:    dumpPeriod,
		pprofFilePath: pprofFilePath,
		logger:        logger,
		manager:       manager,
	}
}

func (c *PprofTaskCommandImpl) GetEvent() string {
	return c.events
}

func (c *PprofTaskCommandImpl) GetCreateTime() int64 {
	return c.createTime
}

func (c *PprofTaskCommandImpl) GetDuration() time.Duration {
	return c.duration
}

func (command *PprofTaskCommandImpl) StartTask() (io.Writer, error) {
	var err error
	var writer io.Writer

	// For CPU profiling, check global state first
	if command.events == reporter.EventsTypeCPU && !isRunning.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("CPU profiling is already running")
	}
	if command.pprofFilePath == "" {
		// sample data to buffer
		writer = &bytes.Buffer{}
	} else {
		// sample data to file
		timestamp := time.Now().Format("20060102150405")
		fileName := strings.ToLower(command.events) + "_" + timestamp + "_" + command.taskId + ".pprof"
		pprofFilePath := command.pprofFilePath + fileName
		writer, err = os.Create(pprofFilePath)
		if err != nil {
			if command.GetEvent() == reporter.EventsTypeCPU {
				isRunning.Store(false)
			}
			return nil, err
		}
	}

	switch command.events {
	case reporter.EventsTypeCPU:
		if err = pprof.StartCPUProfile(writer); err != nil {
			isRunning.Store(false)
			return nil, err
		}
		command.logger.Infof("CPU profiling task started for %s", command.taskId)
	case reporter.EventsTypeHeap:
		command.logger.Infof("Heap profiling task started for %s", command.taskId)
	case reporter.EventsTypeBlock:
		runtime.SetBlockProfileRate(command.dumpPeriod)
		command.logger.Infof("Block profiling task started for %s", command.taskId)
	case reporter.EventsTypeMutex:
		runtime.SetMutexProfileFraction(command.dumpPeriod)
		command.logger.Infof("Mutex profiling task started for %s", command.taskId)
	default:
		return nil, fmt.Errorf("unsupported profile type: %s", command.events)
	}

	return writer, nil
}

func (command *PprofTaskCommandImpl) StopTask(writer io.Writer) {

	switch command.events {
	case reporter.EventsTypeCPU:
		pprof.StopCPUProfile()
		isRunning.Store(false)
	case reporter.EventsTypeHeap:
		if err := pprof.WriteHeapProfile(writer); err != nil {
			command.logger.Errorf("write Heap profile error %v", err)
		}
	case reporter.EventsTypeBlock:
		if profile := pprof.Lookup("block"); profile != nil {
			if err := profile.WriteTo(writer, 0); err != nil {
				command.logger.Errorf("write block profile error %v", err)
			}
		}
		runtime.SetBlockProfileRate(0)
	case reporter.EventsTypeMutex:
		if profile := pprof.Lookup("mutex"); profile != nil {
			if err := profile.WriteTo(writer, 0); err != nil {
				command.logger.Errorf("write mutex profile error %v", err)
			}
		}
		runtime.SetMutexProfileFraction(0)
	}

	if command.pprofFilePath != "" {
		if file, ok := (writer).(*os.File); ok {
			if err := file.Close(); err != nil {
				command.logger.Errorf("failed to close pprof file: %v", err)
			}
		}
	}
	command.readPprofData(command.taskId, writer)
	command.logger.Infof("Pprof task completed for taskId: %s, type: %s", command.taskId, command.events)

}

func (command *PprofTaskCommandImpl) readPprofData(taskId string, writer io.Writer) {
	var data []byte
	if command.pprofFilePath == "" {
		if buf, ok := writer.(*bytes.Buffer); ok {
			data = buf.Bytes()
		}
	} else {
		if file, ok := writer.(*os.File); ok {
			filePath := file.Name()
			fileData, err := os.ReadFile(filePath)
			if err != nil {
				command.logger.Errorf("failed to read pprof file %s: %v", filePath, err)
			}
			data = fileData
			if err := os.Remove(filePath); err != nil {
				command.logger.Errorf("failed to remove pprof file %s: %v", filePath, err)
			}
		}
	}
	command.manager.ReportPprof(taskId, data)
}
