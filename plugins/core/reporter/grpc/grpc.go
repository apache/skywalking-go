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
	"fmt"
	"io"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	profilev3 "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
	logv3 "skywalking.apache.org/repo/goapi/collect/logging/v3"
	managementv3 "skywalking.apache.org/repo/goapi/collect/management/v3"
)

const (
	maxSendQueueSize int32 = 30000
)

// NewGRPCReporter create a new reporter to send data to gRPC oap server. Only one backend address is allowed.
func NewGRPCReporter(logger operator.LogOperator,
	serverAddr string,
	checkInterval time.Duration,
	profileFetchIntervalVal time.Duration,
	connManager *reporter.ConnectionManager,
	cdsManager *reporter.CDSManager,
	opts ...ReporterOption,
) (reporter.Reporter, error) {
	r := &gRPCReporter{
		logger:                  logger,
		serverAddr:              serverAddr,
		tracingSendCh:           make(chan *agentv3.SegmentObject, maxSendQueueSize),
		metricsSendCh:           make(chan []*agentv3.MeterData, maxSendQueueSize),
		logSendCh:               make(chan *logv3.LogData, maxSendQueueSize),
		checkInterval:           checkInterval,
		profileFetchIntervalVal: profileFetchIntervalVal,
		connManager:             connManager,
		cdsManager:              cdsManager,
	}
	for _, o := range opts {
		o(r)
	}

	conn, err := connManager.GetConnection(serverAddr)
	if err != nil {
		return nil, err
	}
	r.traceClient = agentv3.NewTraceSegmentReportServiceClient(conn)
	r.metricsClient = agentv3.NewMeterReportServiceClient(conn)
	r.logClient = logv3.NewLogReportServiceClient(conn)
	r.managementClient = managementv3.NewManagementServiceClient(conn)
	r.profileTaskClient = profilev3.NewProfileTaskClient(conn)
	r.profileManager = reporter.NewProfileManager()
	return r, nil
}

type gRPCReporter struct {
	entity                  *reporter.Entity
	serverAddr              string
	logger                  operator.LogOperator
	tracingSendCh           chan *agentv3.SegmentObject
	metricsSendCh           chan []*agentv3.MeterData
	logSendCh               chan *logv3.LogData
	traceClient             agentv3.TraceSegmentReportServiceClient
	metricsClient           agentv3.MeterReportServiceClient
	logClient               logv3.LogReportServiceClient
	managementClient        managementv3.ManagementServiceClient
	profileTaskClient       profilev3.ProfileTaskClient
	profileManager          *reporter.ProfileManager
	checkInterval           time.Duration
	profileFetchIntervalVal time.Duration
	// bootFlag is set if Boot be executed
	bootFlag    bool
	transform   *reporter.Transform
	connManager *reporter.ConnectionManager
	cdsManager  *reporter.CDSManager
}

func (r *gRPCReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
	r.entity = entity
	r.transform = reporter.NewTransform(entity)
	r.initSendPipeline()
	r.check()
	r.fetchProfileTasks()
	r.reportProfileResult()
	r.cdsManager.InitCDS(entity, cdsWatchers)
	r.bootFlag = true
}

func (r *gRPCReporter) ConnectionStatus() reporter.ConnectionStatus {
	return r.connManager.GetConnectionStatus(r.serverAddr)
}

func (r *gRPCReporter) SendTracing(spans []reporter.ReportedSpan) {
	segmentObject := r.transform.TransformSegmentObject(spans)
	if segmentObject == nil {
		return
	}
	defer func() {
		// recover the panic caused by close tracingSendCh
		if err := recover(); err != nil {
			r.logger.Errorf("reporter segment err %v", err)
		}
	}()
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
	} else {
		r.closeGRPCConn()
	}
}

func (r *gRPCReporter) closeGRPCConn() {
	if err := r.connManager.ReleaseConnection(r.serverAddr); err != nil {
		r.logger.Error(err)
	}
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
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			stream, err := r.traceClient.Collect(metadata.NewOutgoingContext(context.Background(), r.connManager.GetMD()))
			if err != nil {
				r.logger.Errorf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			for s := range r.tracingSendCh {
				err = stream.Send(s)
				if err != nil {
					r.logger.Errorf("send segment error %v", err)
					r.closeTracingStream(stream)
					continue StreamLoop
				}
			}
			r.closeTracingStream(stream)
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
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			stream, err := r.metricsClient.CollectBatch(metadata.NewOutgoingContext(context.Background(), r.connManager.GetMD()))
			if err != nil {
				r.logger.Errorf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			for s := range r.metricsSendCh {
				err = stream.Send(&agentv3.MeterDataCollection{
					MeterData: s,
				})
				if err != nil {
					r.logger.Errorf("send metrics error %v", err)
					r.closeMetricsStream(stream)
					continue StreamLoop
				}
			}
			r.closeMetricsStream(stream)
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
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			stream, err := r.logClient.Collect(metadata.NewOutgoingContext(context.Background(), r.connManager.GetMD()))
			if err != nil {
				r.logger.Errorf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			for s := range r.logSendCh {
				err = stream.Send(s)
				if err != nil {
					r.logger.Errorf("send log error %v", err)
					r.closeLogStream(stream)
					continue StreamLoop
				}
			}
			r.closeLogStream(stream)
			break
		}
	}()
}

func (r *gRPCReporter) closeTracingStream(stream agentv3.TraceSegmentReportService_CollectClient) {
	_, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		r.logger.Errorf("send closing error %v", err)
	}
}

func (r *gRPCReporter) closeMetricsStream(stream agentv3.MeterReportService_CollectBatchClient) {
	_, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		r.logger.Errorf("send closing error %v", err)
	}
}

func (r *gRPCReporter) closeLogStream(stream logv3.LogReportService_CollectClient) {
	_, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		r.logger.Errorf("send closing error %v", err)
	}
}

func (r *gRPCReporter) reportInstanceProperties() (err error) {
	_, err = r.managementClient.ReportInstanceProperties(
		metadata.NewOutgoingContext(context.Background(), r.connManager.GetMD()),
		&managementv3.InstanceProperties{
			Service:         r.entity.ServiceName,
			ServiceInstance: r.entity.ServiceInstanceName,
			Properties:      r.entity.Props,
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
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case reporter.ConnectionStatusShutdown:
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(r.checkInterval)
				continue
			}

			if !instancePropertiesSubmitted {
				err := r.reportInstanceProperties()
				if err != nil {
					r.logger.Errorf("report serviceInstance properties error %v", err)
					time.Sleep(r.checkInterval)
					continue
				}
				instancePropertiesSubmitted = true
			}

			_, err := r.managementClient.KeepAlive(
				metadata.NewOutgoingContext(context.Background(), r.connManager.GetMD()),
				&managementv3.InstancePingPkg{
					Service:         r.entity.ServiceName,
					ServiceInstance: r.entity.ServiceInstanceName,
				})

			if err != nil {
				r.logger.Errorf("send keep alive signal error %v", err)
			}
			time.Sleep(r.checkInterval)
		}
	}()
}
func (r *gRPCReporter) fetchProfileTasks() {
	if r.profileFetchIntervalVal < 0 || r.profileTaskClient == nil {
		fmt.Println("profile init error")
		return
	}
	go func() {
		for {
			// 构造请求

			req := &profilev3.ProfileTaskCommandQuery{
				Service:         r.entity.ServiceName,
				ServiceInstance: r.entity.ServiceInstanceName,
			}
			// 拉取任务
			fmt.Println("开始拉取任务")
			resp, err := r.profileTaskClient.GetProfileTaskCommands(context.Background(), req)
			if err != nil {
				r.logger.Errorf("fetch profile task error: %v", err)
				time.Sleep(r.profileFetchIntervalVal)
				continue
			}
			// 处理返回的所有命令
			for _, cmd := range resp.Commands {
				r.handleProfileTask(cmd)
			}
			// 除去已完成的
			r.profileManager.RemoveProfileTask()
			time.Sleep(r.profileFetchIntervalVal)
		}
	}()
}
func (r *gRPCReporter) handleProfileTask(cmd *common.Command) {
	if cmd.Command != "ProfileTaskQuery" {
		return
	}
	r.profileManager.AddProfileTask(cmd.Args)

}
func (r *gRPCReporter) Profiling(traceId string, endPoint string) {
	if r.profileManager != nil {
		r.profileManager.ToProfile(endPoint, traceId)
	}
}
func (r *gRPCReporter) EndProfiling() {
	if r.profileManager != nil {
		r.profileManager.EndProfiling()
	}
}

func (r *gRPCReporter) AddSpanIdToProfile(spanId int32) {
	if r.profileManager.IfProfiling() {
		r.profileManager.AddSpanId(spanId)
	} else {
		fmt.Println("no profile")
	}
}
func (r *gRPCReporter) reportProfileResult() {
	go func() {
		for task := range r.profileManager.ReportResults {
			fmt.Printf("收到任务结果：%+v\n", task)
		}
	}()

}
