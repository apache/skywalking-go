package reporter

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/apache/skywalking-go/plugins/core/operator"
	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	pprofv10 "skywalking.apache.org/repo/goapi/collect/pprof/v10"
)

const (
	// Pprof event types
	EventsTypeCPU   = "cpu"
	EventsTypeHeap  = "heap"
	EventsTypeBlock = "block"
	EventsTypeMutex = "mutex"
	// max chunk size for pprof data
	maxChunkSize = 1 * 1024 * 1024
)

type PprofTaskCommand interface {
	GetEvent() string
	GetCreateTime() int64
	GetDuration() time.Duration
	StartTask() (io.Writer, error)
	StopTask(io.Writer)
}
type PprofReporter interface {
	ReportPprof(taskId string, content []byte)
}

var NewPprofTaskCommand func(serialNumber, taskId, events string, duration time.Duration, createTime int64, dumpPeriod int, pprofFilePath string, logger operator.LogOperator, manager PprofReporter) PprofTaskCommand

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
}

func NewPprofTaskManager(logger operator.LogOperator, serverAddr string, pprofInterval time.Duration, connManager *ConnectionManager, pprofFilePath string) (*PprofTaskManager, error) {
	PprofManager := &PprofTaskManager{
		logger:        logger,
		serverAddr:    serverAddr,
		pprofInterval: pprofInterval,
		connManager:   connManager,
		pprofFilePath: pprofFilePath,
	}
	if pprofInterval > 0 {
		conn, err := connManager.GetConnection(serverAddr)
		if err != nil {
			return nil, err
		}
		PprofManager.PprofClient = pprofv10.NewPprofTaskClient(conn)
		PprofManager.commands = nil
	}
	return PprofManager, nil
}

func (r *PprofTaskManager) InitPprofTask(entity *Entity) {
	if r.PprofClient == nil {
		return
	}
	r.entity = entity
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

func (r *PprofTaskManager) HandleCommand(rawCommand *commonv3.Command) error {
	command := r.deserializePprofTaskCommand(rawCommand)
	if command.GetCreateTime() > r.LastUpdateTime {
		r.LastUpdateTime = command.GetCreateTime()
	}

	if command.GetEvent() == EventsTypeHeap {
		// direct sampling of Heap
		writer, err := command.StartTask()
		if err != nil {
			r.logger.Errorf("start %s pprof error %v \n", command.GetEvent(), err)
			return err
		}
		command.StopTask(writer)

	} else {
		// The CPU, Block, and Mutex sampling lasts for a duration and then stops
		writer, err := command.StartTask()
		if err != nil {
			r.logger.Errorf("start CPU pprof error %v \n", err)
			return err
		}
		time.AfterFunc(command.GetDuration(), func() {
			command.StopTask(writer)
		})
	}

	return nil
}

func (r *PprofTaskManager) deserializePprofTaskCommand(command *commonv3.Command) PprofTaskCommand {
	args := command.Args
	taskId := ""
	serialNumber := ""
	events := ""
	duration := 0
	dumpPeriod := 0 // Use -1 to indicate no explicit value provided
	var createTime int64 = 0
	for _, pair := range args {
		if pair.GetKey() == "SerialNumber" {
			serialNumber = pair.GetValue()
		} else if pair.GetKey() == "TaskId" {
			taskId = pair.GetValue()
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
		serialNumber,
		taskId,
		events,
		time.Duration(duration)*time.Minute,
		createTime,
		dumpPeriod,
		r.pprofFilePath,
		r.logger,
		r,
	)
}

func (r *PprofTaskManager) ReportPprof(taskId string, content []byte) {
	metaData := &pprofv10.PprofMetaData{
		Service:         r.entity.ServiceName,
		ServiceInstance: r.entity.ServiceInstanceName,
		TaskId:          taskId,
		Type:            pprofv10.PprofProfilingStatus_PPROF_PROFILING_SUCCESS,
		ContentSize:     int32(len(content)),
	}

	go r.uploadPprofData(metaData, content, taskId)
}

func (r *PprofTaskManager) uploadPprofData(metaData *pprofv10.PprofMetaData, content []byte, taskId string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := r.PprofClient.Collect(ctx)
	if err != nil {
		r.logger.Errorf("failed to start collect stream: %v", err)
		return
	}

	// Send metadata
	metadataMsg := &pprofv10.PprofData{
		Metadata: metaData,
	}
	if err := stream.Send(metadataMsg); err != nil {
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
	default:
	}

	// Upload content in chunks
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
			r.logger.Errorf("context timeout during chunk upload for task %s", taskId)
			return
		default:
		}
	}

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
			r.logger.Errorf("error receiving final response for task %s: %v", taskId, err)
			break
		}
	}
}
