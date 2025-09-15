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

package reporter

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/apache/skywalking-go/plugins/core/operator"
	commonv3 "github.com/apache/skywalking-go/protocols/collect/common/v3"
	pprofv10 "github.com/apache/skywalking-go/protocols/collect/pprof/v10"
)

const (
	// Pprof event types
	PprofEventsTypeCPU       = "cpu"
	PprofEventsTypeHeap      = "heap"
	PprofEventsTypeAllocs    = "allocs"
	PprofEventsTypeBlock     = "block"
	PprofEventsTypeMutex     = "mutex"
	PprofEventsTypeThread    = "threadcreate"
	PprofEventsTypeGoroutine = "goroutine"
	// max chunk size for pprof data
	maxChunkSize = 1 * 1024 * 1024
	// max send queue size for pprof data
	maxPprofSendQueueSize = 30000
)

type PprofTaskCommand interface {
	GetEvent() string
	GetCreateTime() int64
	GetDuration() time.Duration
	StartTask() (io.Writer, error)
	StopTask(io.Writer)
}
type PprofReporter interface {
	ReportPprof(taskID string, content []byte)
}

var NewPprofTaskCommand func(taskID, events string, duration time.Duration,
	createTime int64, dumpPeriod int, pprofFilePath string,
	logger operator.LogOperator, manager PprofReporter) PprofTaskCommand

type PprofTaskManager struct {
	logger         operator.LogOperator
	serverAddr     string
	pprofInterval  time.Duration
	PprofClient    pprofv10.PprofTaskClient // for grpc
	connManager    *ConnectionManager
	entity         *Entity
	pprofFilePath  string
	LastUpdateTime int64
	commands       PprofTaskCommand
	pprofSendCh    chan *pprofv10.PprofData
}

func NewPprofTaskManager(logger operator.LogOperator, serverAddr string,
	pprofInterval time.Duration, connManager *ConnectionManager,
	pprofFilePath string) (*PprofTaskManager, error) {
	pprofManager := &PprofTaskManager{
		logger:        logger,
		serverAddr:    serverAddr,
		pprofInterval: pprofInterval,
		connManager:   connManager,
		pprofFilePath: pprofFilePath,
		pprofSendCh:   make(chan *pprofv10.PprofData, maxPprofSendQueueSize),
	}
	if pprofInterval > 0 {
		conn, err := connManager.GetConnection(serverAddr)
		if err != nil {
			return nil, err
		}
		pprofManager.PprofClient = pprofv10.NewPprofTaskClient(conn)
		pprofManager.commands = nil
	}
	return pprofManager, nil
}

func (r *PprofTaskManager) InitPprofTask(entity *Entity) {
	if r.PprofClient == nil {
		return
	}
	r.entity = entity
	r.initPprofSendPipeline()
	go func() {
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case ConnectionStatusShutdown:
				return
			case ConnectionStatusDisconnect:
				time.Sleep(r.pprofInterval)
				continue
			}
			pprofCommand, err := r.PprofClient.GetPprofTaskCommands(context.Background(), &pprofv10.PprofTaskCommandQuery{
				Service:         r.entity.ServiceName,
				ServiceInstance: r.entity.ServiceInstanceName,
				LastCommandTime: r.LastUpdateTime,
			})
			if err != nil {
				r.logger.Errorf("fetch pprof task commands error %v", err)
				time.Sleep(r.pprofInterval)
				continue
			}

			if len(pprofCommand.GetCommands()) > 0 && pprofCommand.GetCommands()[0].Command == "PprofTaskQuery" {
				rawCommand := pprofCommand.GetCommands()[0]
				r.HandleCommand(rawCommand)
			}

			time.Sleep(r.pprofInterval)
		}
	}()
}

func (r *PprofTaskManager) HandleCommand(rawCommand *commonv3.Command) {
	command := r.deserializePprofTaskCommand(rawCommand)
	if command.GetCreateTime() > r.LastUpdateTime {
		r.LastUpdateTime = command.GetCreateTime()
	} else {
		return
	}

	if command.GetEvent() == PprofEventsTypeHeap || command.GetEvent() == PprofEventsTypeAllocs ||
		command.GetEvent() == PprofEventsTypeGoroutine || command.GetEvent() == PprofEventsTypeThread {
		// direct sampling of Heap
		writer, err := command.StartTask()
		if err != nil {
			r.logger.Errorf("start %s pprof error %v \n", command.GetEvent(), err)
			return
		}
		command.StopTask(writer)
	} else {
		// The CPU, Block, and Mutex sampling lasts for a duration and then stops
		writer, err := command.StartTask()
		if err != nil {
			r.logger.Errorf("start CPU pprof error %v \n", err)
			return
		}
		time.AfterFunc(command.GetDuration(), func() {
			command.StopTask(writer)
		})
	}
}

func (r *PprofTaskManager) deserializePprofTaskCommand(command *commonv3.Command) PprofTaskCommand {
	args := command.Args
	taskID := ""
	events := ""
	duration := 0
	dumpPeriod := 0 // Use -1 to indicate no explicit value provided
	var createTime int64 = 0
	for _, pair := range args {
		if pair.GetKey() == "TaskId" {
			taskID = pair.GetValue()
		} else if pair.GetKey() == "Events" {
			events = pair.GetValue()
		} else if pair.GetKey() == "Duration" {
			if val, err := strconv.Atoi(pair.GetValue()); err == nil && val > 0 {
				duration = val
			}
		} else if pair.GetKey() == "DumpPeriod" {
			if val, err := strconv.Atoi(pair.GetValue()); err == nil && val >= 0 {
				dumpPeriod = val
			}
		} else if pair.GetKey() == "CreateTime" {
			createTime, _ = strconv.ParseInt(pair.GetValue(), 10, 64)
		}
	}

	return NewPprofTaskCommand(
		taskID,
		events,
		time.Duration(duration)*time.Minute,
		createTime,
		dumpPeriod,
		r.pprofFilePath,
		r.logger,
		r,
	)
}

func (r *PprofTaskManager) ReportPprof(taskID string, content []byte) {
	metaData := &pprofv10.PprofMetaData{
		Service:         r.entity.ServiceName,
		ServiceInstance: r.entity.ServiceInstanceName,
		TaskId:          taskID,
		Type:            pprofv10.PprofProfilingStatus_PPROF_PROFILING_SUCCESS,
		ContentSize:     int32(len(content)),
	}

	pprofData := &pprofv10.PprofData{
		Metadata: metaData,
		Result: &pprofv10.PprofData_Content{
			Content: content,
		},
	}

	defer func() {
		if err := recover(); err != nil {
			r.logger.Errorf("reporter pprof err %v", err)
		}
	}()
	select {
	case r.pprofSendCh <- pprofData:
	default:
		r.logger.Errorf("reach max pprof send buffer")
	}
}

func (r *PprofTaskManager) initPprofSendPipeline() {
	if r.PprofClient == nil {
		return
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("PprofTaskManager initPprofSendPipeline panic err %v", err)
			}
		}()
	StreamLoop:
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case ConnectionStatusShutdown:
				return
			case ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			for pprofData := range r.pprofSendCh {
				r.uploadPprofData(pprofData)
			}
			break
		}
	}()
}

func (r *PprofTaskManager) uploadPprofData(pprofData *pprofv10.PprofData) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := r.PprofClient.Collect(ctx)
	if err != nil {
		r.logger.Errorf("failed to start collect stream: %v", err)
		return
	}

	// Send metadata first
	metadataMsg := &pprofv10.PprofData{
		Metadata: pprofData.Metadata,
	}
	if err = stream.Send(metadataMsg); err != nil {
		r.logger.Errorf("failed to send metadata: %v", err)
		return
	}

	resp, err := stream.Recv()
	if err != nil {
		r.logger.Errorf("failed to receive server response: %v", err)
		return
	}

	switch resp.Status {
	case pprofv10.PprofProfilingStatus_PPROF_TERMINATED_BY_OVERSIZE:
		r.logger.Errorf("pprof is too large to be received by the oap server")
		return
	case pprofv10.PprofProfilingStatus_PPROF_EXECUTION_TASK_ERROR:
		r.logger.Errorf("server rejected pprof upload due to execution task error")
		return
	}

	// Upload content in chunks
	content := pprofData.GetContent()
	chunkCount := 0
	contentSize := len(content)

	for offset := 0; offset < contentSize; offset += maxChunkSize {
		end := offset + maxChunkSize
		if end > contentSize {
			end = contentSize
		}

		chunkData := &pprofv10.PprofData{
			Result: &pprofv10.PprofData_Content{
				Content: content[offset:end],
			},
		}

		if err := stream.Send(chunkData); err != nil {
			r.logger.Errorf("failed to send pprof chunk %d: %v", chunkCount, err)
			return
		}
		chunkCount++
		// Check context timeout
		select {
		case <-ctx.Done():
			r.logger.Errorf("context timeout during chunk upload for task %s", pprofData.Metadata.TaskId)
			return
		default:
		}
	}

	r.closePprofStream(stream)
}
func (r *PprofTaskManager) closePprofStream(stream pprofv10.PprofTask_CollectClient) {
	if err := stream.CloseSend(); err != nil {
		r.logger.Errorf("failed to close send stream: %v", err)
		return
	}

	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			r.logger.Errorf("error receiving final response %v", err)
			break
		}
	}
}
