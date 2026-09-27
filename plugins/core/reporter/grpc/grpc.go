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

package grpc

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	common "github.com/apache/skywalking-go/protocols/collect/common/v3"
	agentv3 "github.com/apache/skywalking-go/protocols/collect/language/agent/v3"
	profilev3 "github.com/apache/skywalking-go/protocols/collect/language/profile/v3"
	logv3 "github.com/apache/skywalking-go/protocols/collect/logging/v3"
	managementv3 "github.com/apache/skywalking-go/protocols/collect/management/v3"
)

const (
	maxSendQueueSize int32 = 30000
)

// NewGRPCReporter create a new reporter to send data to gRPC oap server.
// backend_service may be a single host:port or a comma-separated list; multiple
// addresses are published to gRPC (pick_first) via a static multi-backend resolver.
func NewGRPCReporter(logger operator.LogOperator,
	serverAddr string,
	checkInterval time.Duration,
	profileFetchInterval time.Duration,
	connManager *reporter.ConnectionManager,
	cdsManager *reporter.CDSManager,
	pprofTaskManager *reporter.PprofTaskManager,
	opts ...ReporterOption,
) (reporter.Reporter, error) {
	r := &gRPCReporter{
		logger:               logger,
		serverAddr:           serverAddr,
		tracingSendCh:        make(chan *agentv3.SegmentObject, maxSendQueueSize),
		metricsSendCh:        make(chan []*agentv3.MeterData, maxSendQueueSize),
		logSendCh:            make(chan *logv3.LogData, maxSendQueueSize),
		checkInterval:        checkInterval,
		profileFetchInterval: profileFetchInterval,
		connManager:          connManager,
		cdsManager:           cdsManager,
		pprofTaskManager:     pprofTaskManager,
	}
	for _, o := range opts {
		o(r)
	}
	r.lastProfileCommandTime = -1
	conn, err := connManager.GetConnection(serverAddr)
	if err != nil {
		return nil, err
	}
	r.conn = conn
	r.traceClient = agentv3.NewTraceSegmentReportServiceClient(conn)
	r.metricsClient = agentv3.NewMeterReportServiceClient(conn)
	r.logClient = logv3.NewLogReportServiceClient(conn)
	r.managementClient = managementv3.NewManagementServiceClient(conn)
	r.profileTaskClient = profilev3.NewProfileTaskClient(conn)
	return r, nil
}

type gRPCReporter struct {
	entity               *reporter.Entity
	serverAddr           string
	logger               operator.LogOperator
	tracingSendCh        chan *agentv3.SegmentObject
	metricsSendCh        chan []*agentv3.MeterData
	logSendCh            chan *logv3.LogData
	conn                 *grpc.ClientConn
	traceClient          agentv3.TraceSegmentReportServiceClient
	metricsClient        agentv3.MeterReportServiceClient
	logClient            logv3.LogReportServiceClient
	managementClient     managementv3.ManagementServiceClient
	profileTaskClient    profilev3.ProfileTaskClient
	profileTaskManager   reporter.ProfileTaskManager
	checkInterval        time.Duration
	profileFetchInterval time.Duration
	// lastProfileCommandTime is the last timestamp we used to fetch profile commands.
	lastProfileCommandTime int64
	// bootFlag is set if Boot be executed
	bootFlag         bool
	transform        *reporter.Transform
	connManager      *reporter.ConnectionManager
	cdsManager       *reporter.CDSManager
	pprofTaskManager *reporter.PprofTaskManager
}

func (r *gRPCReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
	r.entity = entity
	r.transform = reporter.NewTransform(entity)
	r.initSendPipeline()
	r.check()
	r.fetchProfileTasks()
	r.cdsManager.InitCDS(entity, cdsWatchers)
	r.pprofTaskManager.InitPprofTask(entity)
	r.bootFlag = true
}

func (r *gRPCReporter) ConnectionStatus() reporter.ConnectionStatus {
	return r.connManager.GetConnectionStatus(r.serverAddr)
}

func (r *gRPCReporter) SendTracing(spans []reporter.ReportedSpan) {
	// The recover must be registered BEFORE the transform call: SendTracing
	// runs on the segment collector goroutine, so a panic escaping from the
	// transform (or the channel send below, e.g. on a closed tracingSendCh)
	// would otherwise kill the whole process.
	defer func() {
		if err := recover(); err != nil {
			r.logger.Errorf("reporter segment err %v", err)
		}
	}()
	segmentObject := r.transform.TransformSegmentObject(spans)
	if segmentObject == nil {
		return
	}
	select {
	case r.tracingSendCh <- segmentObject:
	default:
		r.logger.Errorf("reach max tracing send buffer")
	}
}

func (r *gRPCReporter) SendMetrics(metrics []reporter.ReportedMeter) {
	meters := r.transform.TransformMeterData(metrics)
	if meters == nil {
		return
	}
	defer func() {
		// recover the panic caused by close metricsSendCh
		if err := recover(); err != nil {
			r.logger.Errorf("reporter metrics err %v", err)
		}
	}()
	select {
	case r.metricsSendCh <- meters:
	default:
		r.logger.Errorf("reach max metrics send buffer")
	}
}

func (r *gRPCReporter) SendLog(log *logv3.LogData) {
	defer func() {
		if err := recover(); err != nil {
			r.logger.Errorf("reporter log err %v", err)
		}
	}()
	select {
	case r.logSendCh <- log:
	default:
	}
}

func (r *gRPCReporter) Close() {
	if r.bootFlag {
		if r.tracingSendCh != nil {
			close(r.tracingSendCh)
		}
		if r.metricsSendCh != nil {
			close(r.metricsSendCh)
		}
		if r.logSendCh != nil {
			close(r.logSendCh)
		}
	} else {
		r.closeGRPCConn()
	}
}

func (r *gRPCReporter) closeGRPCConn() {
	if err := r.connManager.ReleaseConnection(r.serverAddr); err != nil {
		r.logger.Error(err)
	}
}

// sendWithRecover invokes send and recovers from a panic raised while encoding or
// transmitting a single message, so that one corrupted payload cannot tear down the
// whole send pipeline. On a recovered panic it logs via the existing logger and
// returns recovered=true, telling the caller to skip the current message and keep
// streaming the rest.
//
// Such a panic originates in protobuf size/marshal computation (the #13885 crash),
// which runs before any bytes are written to the stream, so the stream stays valid
// and may be reused for the next message. Should a panic ever leave the stream
// inconsistent, the following send returns an error and the caller reconnects.
func (r *gRPCReporter) sendWithRecover(send func() error) (recovered bool, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			r.logger.Errorf("gRPCReporter recovered from panic while sending, skip current message: %v", rec)
			recovered = true
		}
	}()
	err = send()
	return recovered, err
}

// pipelineSend wraps stream Send with BoundSend so a half-open connection cannot
// stall the Collect loop forever.
func (r *gRPCReporter) pipelineSend(cancel context.CancelFunc, send func() error) (recovered bool, err error) {
	return r.sendWithRecover(func() error {
		return reporter.BoundSend(cancel, send, 0)
	})
}

// nolint
func (r *gRPCReporter) initSendPipeline() {
	if r.traceClient == nil {
		return
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("gRPCReporter initSendPipeline trace client Collect panic err %v", err)
			}
		}()
	StreamLoop:
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case reporter.ConnectionStatusShutdown:
				return
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			ctx, cancel, stopOpen := reporter.BackendStreamContext(r.serverAddr, r.checkInterval)
			stream, err := r.traceClient.Collect(metadata.NewOutgoingContext(ctx, r.connManager.GetMD()))
			if timedOut := stopOpen(); err != nil || timedOut {
				cancel()
				if err == nil {
					err = ctx.Err()
				}
				r.logger.Errorf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			go reporter.WatchConnCancelOnUnready(ctx, cancel, r.conn)

			for s := range r.tracingSendCh {
				recovered, sendErr := r.pipelineSend(cancel, func() error { return stream.Send(s) })
				if recovered {
					continue
				}
				if sendErr != nil {
					r.logger.Errorf("send segment error %v", sendErr)
					cancel()
					r.closeTracingStream(cancel, stream)
					continue StreamLoop
				}
			}
			cancel()
			r.closeTracingStream(cancel, stream)
			r.closeGRPCConn()
			break
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("gRPCReporter initSendPipeline metrics client CollectBatch panic err %v", err)
			}
		}()
	StreamLoop:
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case reporter.ConnectionStatusShutdown:
				return
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			ctx, cancel, stopOpen := reporter.BackendStreamContext(r.serverAddr, r.checkInterval)
			stream, err := r.metricsClient.CollectBatch(metadata.NewOutgoingContext(ctx, r.connManager.GetMD()))
			if timedOut := stopOpen(); err != nil || timedOut {
				cancel()
				if err == nil {
					err = ctx.Err()
				}
				r.logger.Errorf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			go reporter.WatchConnCancelOnUnready(ctx, cancel, r.conn)

			for s := range r.metricsSendCh {
				recovered, sendErr := r.pipelineSend(cancel, func() error {
					return stream.Send(&agentv3.MeterDataCollection{MeterData: s})
				})
				if recovered {
					continue
				}
				if sendErr != nil {
					// Cancel before CloseAndRecv: a half-open peer can hang
					// CloseAndRecv forever and block reconnect.
					cancel()
					r.logger.Errorf("send metrics error %v", sendErr)
					r.closeMetricsStream(cancel, stream)
					continue StreamLoop
				}
			}
			cancel()
			r.closeMetricsStream(cancel, stream)
			break
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("gRPCReporter initSendPipeline log client Collect panic err %v", err)
			}
		}()
	StreamLoop:
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case reporter.ConnectionStatusShutdown:
				return
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			ctx, cancel, stopOpen := reporter.BackendStreamContext(r.serverAddr, r.checkInterval)
			stream, err := r.logClient.Collect(metadata.NewOutgoingContext(ctx, r.connManager.GetMD()))
			if timedOut := stopOpen(); err != nil || timedOut {
				cancel()
				if err == nil {
					err = ctx.Err()
				}
				r.logger.Errorf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			go reporter.WatchConnCancelOnUnready(ctx, cancel, r.conn)

			for s := range r.logSendCh {
				recovered, sendErr := r.pipelineSend(cancel, func() error { return stream.Send(s) })
				if recovered {
					continue
				}
				if sendErr != nil {
					cancel()
					r.logger.Errorf("send log error %v", sendErr)
					r.closeLogStream(cancel, stream)
					continue StreamLoop
				}
			}
			cancel()
			r.closeLogStream(cancel, stream)
			break
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("gRPCReporter reportProfileResult panic err %v", err)
			}
		}()

	StreamLoop:
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case reporter.ConnectionStatusShutdown:
				return
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			ctx, cancel, stopOpen := reporter.BackendStreamContext(r.serverAddr, r.checkInterval)
			stream, err := r.profileTaskClient.GoProfileReport(metadata.NewOutgoingContext(ctx, r.connManager.GetMD()))
			if timedOut := stopOpen(); err != nil || timedOut {
				cancel()
				if err == nil {
					err = ctx.Err()
				}
				r.logger.Errorf("open profile stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			go reporter.WatchConnCancelOnUnready(ctx, cancel, r.conn)

			re := r.profileTaskManager.GetProfileResults()

			for task := range re {
				profileData := &profilev3.GoProfileData{
					TaskId:  task.TaskID,
					Payload: task.Payload,
					IsLast:  task.IsLast,
				}
				r.logger.Infof("Sending profile task: TaskID='%s', PayloadSize=%d, IsLast=%v",
					task.TaskID, len(task.Payload), task.IsLast)
				recovered, sendErr := r.pipelineSend(cancel, func() error { return stream.Send(profileData) })
				if recovered {
					continue
				}
				if sendErr != nil {
					cancel()
					r.logger.Errorf("send profile data error %v", sendErr)
					r.closeProfileStream(cancel, stream)
					continue StreamLoop
				}
				if task.IsLast {
					r.profileTaskManager.ProfileFinish()
					var report = profilev3.ProfileTaskFinishReport{
						TaskId:          task.TaskID,
						Service:         r.entity.ServiceName,
						ServiceInstance: r.entity.ServiceInstanceName,
					}
					finishCtx, finishCancel := reporter.BackendRPCContext(r.serverAddr, r.checkInterval)
					_, err = r.profileTaskClient.ReportTaskFinish(
						metadata.NewOutgoingContext(finishCtx, r.connManager.GetMD()), &report)
					finishCancel()
					if err != nil {
						r.logger.Errorf("report profile task finish error %v", err)
					}
				}
			}
			cancel()
			r.closeProfileStream(cancel, stream)
			break
		}
	}()
}

// close*Stream runs CloseAndRecv under BoundSend so a half-open peer cannot
// stall StreamLoop (and pick_first failover) after Send already failed.
func (r *gRPCReporter) closeTracingStream(cancel context.CancelFunc, stream agentv3.TraceSegmentReportService_CollectClient) {
	if err := reporter.BoundSend(cancel, func() error {
		_, err := stream.CloseAndRecv()
		if err == io.EOF {
			return nil
		}
		return err
	}, 0); err != nil {
		r.logger.Errorf("send closing error %v", err)
	}
}

func (r *gRPCReporter) closeMetricsStream(cancel context.CancelFunc, stream agentv3.MeterReportService_CollectBatchClient) {
	if err := reporter.BoundSend(cancel, func() error {
		_, err := stream.CloseAndRecv()
		if err == io.EOF {
			return nil
		}
		return err
	}, 0); err != nil {
		r.logger.Errorf("send closing error %v", err)
	}
}

func (r *gRPCReporter) closeLogStream(cancel context.CancelFunc, stream logv3.LogReportService_CollectClient) {
	if err := reporter.BoundSend(cancel, func() error {
		_, err := stream.CloseAndRecv()
		if err == io.EOF {
			return nil
		}
		return err
	}, 0); err != nil {
		r.logger.Errorf("send closing error %v", err)
	}
}

func (r *gRPCReporter) closeProfileStream(cancel context.CancelFunc, stream profilev3.ProfileTask_GoProfileReportClient) {
	if err := reporter.BoundSend(cancel, func() error {
		_, err := stream.CloseAndRecv()
		if err == io.EOF {
			return nil
		}
		return err
	}, 0); err != nil {
		r.logger.Errorf("send profile closing error %v", err)
	}
}
func (r *gRPCReporter) reportInstanceProperties() (err error) {
	ctx, cancel := reporter.BackendRPCContext(r.serverAddr, r.checkInterval)
	defer cancel()
	_, err = r.managementClient.ReportInstanceProperties(
		metadata.NewOutgoingContext(ctx, r.connManager.GetMD()),
		&managementv3.InstanceProperties{
			Service:         r.entity.ServiceName,
			ServiceInstance: r.entity.ServiceInstanceName,
			Properties:      r.entity.Props,
		})
	return err
}

func (r *gRPCReporter) sendKeepAlive() error {
	ctx, cancel := reporter.BackendRPCContext(r.serverAddr, r.checkInterval)
	defer cancel()
	_, err := r.managementClient.KeepAlive(
		metadata.NewOutgoingContext(ctx, r.connManager.GetMD()),
		&managementv3.InstancePingPkg{
			Service:         r.entity.ServiceName,
			ServiceInstance: r.entity.ServiceInstanceName,
		})
	return err
}

func (r *gRPCReporter) check() {
	if r.checkInterval < 0 || r.managementClient == nil {
		return
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("gRPCReporter check panic err %v", err)
			}
		}()
		instancePropertiesSubmitted := false
		propertyRefreshHeartbeats := 0
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case reporter.ConnectionStatusShutdown:
				return
			case reporter.ConnectionStatusDisconnect:
				// Re-register after a transport outage so a pick_first standby
				// that does not share instance metadata still learns this agent.
				instancePropertiesSubmitted = false
				time.Sleep(r.checkInterval)
				continue
			}

			if !instancePropertiesSubmitted || propertyRefreshHeartbeats >= 10 {
				err := r.reportInstanceProperties()
				if err != nil {
					// Keep heartbeats flowing; a properties failure must not
					// stall the management loop or block failover recovery.
					r.logger.Errorf("report serviceInstance properties error %v", err)
				} else {
					instancePropertiesSubmitted = true
					propertyRefreshHeartbeats = 0
				}
			}

			if err := r.sendKeepAlive(); err != nil {
				r.logger.Errorf("send keep alive signal error %v", err)
			}
			propertyRefreshHeartbeats++
			time.Sleep(r.checkInterval)
		}
	}()
}

func (r *gRPCReporter) fetchProfileTasks() {
	if r.profileFetchInterval < 0 {
		r.logger.Errorf("profile init error:profileFetchInterval is %v", r.profileFetchInterval)
		return
	}
	go func() {
		for {
			// The recover wraps a single iteration: this long-lived goroutine
			// has no other protection and a panic while handling the profile
			// commands would otherwise kill the whole process.
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						r.logger.Errorf("gRPCReporter recovered from panic while fetching profile tasks: %v", rec)
					}
				}()
				r.fetchProfileTasksOnce()
			}()
			time.Sleep(r.profileFetchInterval)
		}
	}()
}

// fetchProfileTasksOnce pulls and handles the pending profile task commands of
// one polling round.
func (r *gRPCReporter) fetchProfileTasksOnce() {
	// Construct the request
	req := &profilev3.ProfileTaskCommandQuery{
		Service:         r.entity.ServiceName,
		ServiceInstance: r.entity.ServiceInstanceName,
		LastCommandTime: r.lastProfileCommandTime,
	}

	ctx, cancel := reporter.BackendRPCContext(r.serverAddr, r.profileFetchInterval)
	defer cancel()
	if r.profileTaskClient == nil {
		return
	}
	resp, err := r.profileTaskClient.GetProfileTaskCommands(ctx, req)
	if err != nil {
		r.logger.Errorf("fetch profile task error: %v", err)
		return
	}

	// Handle all returned commands
	for _, cmd := range resp.Commands {
		nt := r.handleProfileTask(cmd, r.lastProfileCommandTime)
		if nt > r.lastProfileCommandTime {
			r.lastProfileCommandTime = nt
		}
	}

	// Remove completed tasks
	r.profileTaskManager.RemoveProfileTask()
}

func (r *gRPCReporter) AddProfileTaskManager(p reporter.ProfileTaskManager) {
	r.profileTaskManager = p
}

func (r *gRPCReporter) handleProfileTask(cmd *common.Command, t int64) int64 {
	if cmd.Command != "ProfileTaskQuery" {
		return t
	}
	nt := r.profileTaskManager.AddProfileTask(cmd.Args, t)
	return nt
}
