// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package core

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
)

// CPU profiling state to ensure only one CPU profiling task runs at a time
var profilingIsRunning atomic.Bool

func init() {
	reporter.NewPprofTaskCommand = NewPprofTaskCommand
}

type PprofTaskCommandImpl struct {
	// Pprof Task ID
	taskID string
	// Type of profiling (CPU/Heap/Block/Mutex)
	events string
	// unit is minute
	duration time.Duration
	// Unix timestamp in milliseconds when the task was created
	createTime int64
	dumpPeriod int

	// for pprof task service
	pprofFilePath string
	logger        operator.LogOperator
	manager       reporter.PprofReporter
}

func NewPprofTaskCommand(taskID, events string, duration time.Duration,
	createTime int64, dumpPeriod int, pprofFilePath string,
	logger operator.LogOperator, manager reporter.PprofReporter) reporter.PprofTaskCommand {
	return &PprofTaskCommandImpl{
		taskID:        taskID,
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

func (c *PprofTaskCommandImpl) closeFileWriter(writer io.Writer) {
	if file, ok := writer.(*os.File); ok {
		if err := file.Close(); err != nil {
			c.logger.Errorf("failed to close pprof file: %v", err)
		}
	}
}

func (c *PprofTaskCommandImpl) getWriter() (io.Writer, error) {
	// sample data to buffer
	if c.pprofFilePath == "" {

		return &bytes.Buffer{}, nil
	}

	// sample data to file
	pprofFileName := filepath.Join(c.taskID, ".pprof")
	pprofFilePath := filepath.Join(c.pprofFilePath, pprofFileName)
	if err := os.MkdirAll(filepath.Dir(pprofFilePath), os.ModePerm); err != nil {
		return nil, err
	}

	writer, err := os.Create(pprofFilePath)
	if err != nil {
		return nil, err
	}

	return writer, nil
}

func (c *PprofTaskCommandImpl) StartTask() (io.Writer, error) {
	// For CPU profiling, check global state first
	if c.events == reporter.PprofEventsTypeCPU && !profilingIsRunning.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("CPU profiling is already running")
	}

	writer, err := c.getWriter()
	if err != nil {
		if c.events == reporter.PprofEventsTypeCPU {
			profilingIsRunning.Store(false)
		}
		return nil, err
	}

	switch c.events {
	case reporter.PprofEventsTypeCPU:
		if err = pprof.StartCPUProfile(writer); err != nil {
			profilingIsRunning.Store(false)
			if c.pprofFilePath != "" {
				c.closeFileWriter(writer)
			}
			return nil, err
		}
	case reporter.PprofEventsTypeBlock:
		runtime.SetBlockProfileRate(c.dumpPeriod)
	case reporter.PprofEventsTypeMutex:
		runtime.SetMutexProfileFraction(c.dumpPeriod)
	case reporter.PprofEventsTypeHeap:
		runtime.MemProfileRate = c.dumpPeriod
	case reporter.PprofEventsTypeAllocs:
		runtime.MemProfileRate = c.dumpPeriod
	}

	return writer, nil
}

func (c *PprofTaskCommandImpl) StopTask(writer io.Writer) {
	switch c.events {
	case reporter.PprofEventsTypeCPU:
		pprof.StopCPUProfile()
		profilingIsRunning.Store(false)
	case reporter.PprofEventsTypeBlock:
		if err := pprof.Lookup("block").WriteTo(writer, 0); err != nil {
			c.logger.Errorf("write Block profile error %v", err)
		}
		runtime.SetBlockProfileRate(0)
	case reporter.PprofEventsTypeMutex:
		if err := pprof.Lookup("mutex").WriteTo(writer, 0); err != nil {
			c.logger.Errorf("write Mutex profile error %v", err)
		}
		runtime.SetMutexProfileFraction(0)
	case reporter.PprofEventsTypeHeap:
		if err := pprof.Lookup("heap").WriteTo(writer, 0); err != nil {
			c.logger.Errorf("write Heap profile error %v", err)
		}
	case reporter.PprofEventsTypeAllocs:
		if err := pprof.Lookup("allocs").WriteTo(writer, 0); err != nil {
			c.logger.Errorf("write Alloc profile error %v", err)
		}
	case reporter.PprofEventsTypeGoroutine:
		if err := pprof.Lookup("goroutine").WriteTo(writer, 0); err != nil {
			c.logger.Errorf("write Goroutine profile error %v", err)
		}
	case reporter.PprofEventsTypeThread:
		if err := pprof.Lookup("threadcreate").WriteTo(writer, 0); err != nil {
			c.logger.Errorf("write Thread profile error %v", err)
		}
	}

	if c.pprofFilePath != "" {
		c.closeFileWriter(writer)
	}
	c.readPprofData(c.taskID, writer)
}

func (c *PprofTaskCommandImpl) readPprofData(taskID string, writer io.Writer) {
	var data []byte
	if c.pprofFilePath == "" {
		if buf, ok := writer.(*bytes.Buffer); ok {
			data = buf.Bytes()
		}
	} else {
		if file, ok := writer.(*os.File); ok {
			filePath := file.Name()
			fileData, err := os.ReadFile(filePath)
			if err != nil {
				c.logger.Errorf("failed to read pprof file %s: %v", filePath, err)
			}
			data = fileData
			if err := os.Remove(filePath); err != nil {
				c.logger.Errorf("failed to remove pprof file %s: %v", filePath, err)
			}
		}
	}
	c.manager.ReportPprof(taskID, data)
}
