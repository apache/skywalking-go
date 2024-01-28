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
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	configuration "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	logv3 "skywalking.apache.org/repo/goapi/collect/logging/v3"
	managementv3 "skywalking.apache.org/repo/goapi/collect/management/v3"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
)

const (
	maxSendQueueSize     int32 = 30000
	defaultCheckInterval       = 20 * time.Second
	defaultCDSInterval         = 20 * time.Second
)

// NewGRPCReporter create a new reporter to send data to gRPC oap server. Only one backend address is allowed.
func NewGRPCReporter(logger operator.LogOperator, serverAddr string, opts ...ReporterOption) (reporter.Reporter, error) {
	r := &gRPCReporter{
		logger:           logger,
		tracingSendCh:    make(chan *agentv3.SegmentObject, maxSendQueueSize),
		metricsSendCh:    make(chan []*agentv3.MeterData, maxSendQueueSize),
		logSendCh:        make(chan *logv3.LogData, maxSendQueueSize),
		checkInterval:    defaultCheckInterval,
		cdsInterval:      defaultCDSInterval, // cds default on
		connectionStatus: reporter.ConnectionStatusConnected,
	}
	for _, o := range opts {
		o(r)
	}

	var credsDialOption grpc.DialOption
	if r.creds != nil {
		// use tls
		credsDialOption = grpc.WithTransportCredentials(r.creds)
	} else {
		credsDialOption = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.Dial(serverAddr, credsDialOption, grpc.WithConnectParams(grpc.ConnectParams{
		// update the max backoff delay interval
		Backoff: backoff.Config{
			BaseDelay:  1.0 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   r.checkInterval,
		},
	}))
	if err != nil {
		return nil, err
	}
	r.conn = conn
	r.traceClient = agentv3.NewTraceSegmentReportServiceClient(r.conn)
	r.metricsClient = agentv3.NewMeterReportServiceClient(r.conn)
	r.logClient = logv3.NewLogReportServiceClient(r.conn)
	r.managementClient = managementv3.NewManagementServiceClient(r.conn)
	if r.cdsInterval > 0 {
		r.cdsClient = configuration.NewConfigurationDiscoveryServiceClient(r.conn)
		r.cdsService = reporter.NewConfigDiscoveryService()
	}
	return r, nil
}

type gRPCReporter struct {
	entity           *reporter.Entity
	logger           operator.LogOperator
	tracingSendCh    chan *agentv3.SegmentObject
	metricsSendCh    chan []*agentv3.MeterData
	logSendCh        chan *logv3.LogData
	conn             *grpc.ClientConn
	traceClient      agentv3.TraceSegmentReportServiceClient
	metricsClient    agentv3.MeterReportServiceClient
	logClient        logv3.LogReportServiceClient
	managementClient managementv3.ManagementServiceClient
	checkInterval    time.Duration
	cdsInterval      time.Duration
	cdsService       *reporter.ConfigDiscoveryService
	cdsClient        configuration.ConfigurationDiscoveryServiceClient

	md    metadata.MD
	creds credentials.TransportCredentials

	// bootFlag is set if Boot be executed
	bootFlag         bool
	connectionStatus reporter.ConnectionStatus
}

func (r *gRPCReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
	r.entity = entity
	r.initSendPipeline()
	r.check()
	r.initCDS(cdsWatchers)
	r.bootFlag = true
}

func (r *gRPCReporter) ConnectionStatus() reporter.ConnectionStatus {
	return r.connectionStatus
}

func (r *gRPCReporter) SendTracing(spans []reporter.ReportedSpan) {
	spanSize := len(spans)
	if spanSize < 1 {
		return
	}
	rootSpan := spans[spanSize-1]
	rootCtx := rootSpan.Context()
	segmentObject := &agentv3.SegmentObject{
		TraceId:         rootCtx.GetTraceID(),
		TraceSegmentId:  rootCtx.GetSegmentID(),
		Spans:           make([]*agentv3.SpanObject, spanSize),
		Service:         r.entity.ServiceName,
		ServiceInstance: r.entity.ServiceInstanceName,
	}
	for i, s := range spans {
		spanCtx := s.Context()
		segmentObject.Spans[i] = &agentv3.SpanObject{
			SpanId:        spanCtx.GetSpanID(),
			ParentSpanId:  spanCtx.GetParentSpanID(),
			StartTime:     s.StartTime(),
			EndTime:       s.EndTime(),
			OperationName: s.OperationName(),
			Peer:          s.Peer(),
			SpanType:      s.SpanType(),
			SpanLayer:     s.SpanLayer(),
			ComponentId:   s.ComponentID(),
			IsError:       s.IsError(),
			Tags:          s.Tags(),
			Logs:          s.Logs(),
		}
		srr := make([]*agentv3.SegmentReference, 0)
		if i == (spanSize-1) && spanCtx.GetParentSpanID() > -1 {
			srr = append(srr, &agentv3.SegmentReference{
				RefType:               agentv3.RefType_CrossThread,
				TraceId:               spanCtx.GetTraceID(),
				ParentTraceSegmentId:  spanCtx.GetParentSegmentID(),
				ParentSpanId:          spanCtx.GetParentSpanID(),
				ParentService:         r.entity.ServiceName,
				ParentServiceInstance: r.entity.ServiceInstanceName,
			})
		}
		if len(s.Refs()) > 0 {
			for _, tc := range s.Refs() {
				srr = append(srr, &agentv3.SegmentReference{
					RefType:                  agentv3.RefType_CrossProcess,
					TraceId:                  spanCtx.GetTraceID(),
					ParentTraceSegmentId:     tc.GetParentSegmentID(),
					ParentSpanId:             tc.GetParentSpanID(),
					ParentService:            tc.GetParentService(),
					ParentServiceInstance:    tc.GetParentServiceInstance(),
					ParentEndpoint:           tc.GetParentEndpoint(),
					NetworkAddressUsedAtPeer: tc.GetAddressUsedAtClient(),
				})
			}
		}
		segmentObject.Spans[i].Refs = srr
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
	if len(metrics) == 0 {
		return
	}
	meters := make([]*agentv3.MeterData, len(metrics))
	for i, m := range metrics {
		meter := &agentv3.MeterData{}
		switch data := m.(type) {
		case reporter.ReportedMeterSingleValue:
			meter.Metric = &agentv3.MeterData_SingleValue{
				SingleValue: &agentv3.MeterSingleValue{
					Name:   data.Name(),
					Labels: r.convertLabels(data.Labels()),
					Value:  data.Value(),
				},
			}
		case reporter.ReportedMeterHistogram:
			buckets := make([]*agentv3.MeterBucketValue, len(data.BucketValues()))
			for i, b := range data.BucketValues() {
				buckets[i] = &agentv3.MeterBucketValue{
					Bucket:             b.Bucket(),
					Count:              b.Count(),
					IsNegativeInfinity: b.IsNegativeInfinity(),
				}
			}
			meter.Metric = &agentv3.MeterData_Histogram{
				Histogram: &agentv3.MeterHistogram{
					Name:   data.Name(),
					Labels: r.convertLabels(data.Labels()),
					Values: buckets,
				},
			}
		}

		meters[i] = meter
	}

	meters[0].Service = r.entity.ServiceName
	meters[0].ServiceInstance = r.entity.ServiceInstanceName
	meters[0].Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		// recover the panic caused by close tracingSendCh
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

func (r *gRPCReporter) convertLabels(labels map[string]string) []*agentv3.Label {
	if len(labels) == 0 {
		return nil
	}
	ls := make([]*agentv3.Label, 0)
	for k, v := range labels {
		ls = append(ls, &agentv3.Label{
			Name:  k,
			Value: v,
		})
	}
	return ls
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
	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			r.logger.Error(err)
		}
	}
}

// nolint
func (r *gRPCReporter) initSendPipeline() {
	if r.traceClient == nil {
		return
	}
	go func() {
	StreamLoop:
		for {
			switch r.updateConnectionStatus() {
			case reporter.ConnectionStatusShutdown:
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			stream, err := r.traceClient.Collect(metadata.NewOutgoingContext(context.Background(), r.md))
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
	StreamLoop:
		for {
			switch r.updateConnectionStatus() {
			case reporter.ConnectionStatusShutdown:
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			stream, err := r.metricsClient.CollectBatch(metadata.NewOutgoingContext(context.Background(), r.md))
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
	StreamLoop:
		for {
			switch r.updateConnectionStatus() {
			case reporter.ConnectionStatusShutdown:
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}

			stream, err := r.logClient.Collect(metadata.NewOutgoingContext(context.Background(), r.md))
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

func (r *gRPCReporter) updateConnectionStatus() reporter.ConnectionStatus {
	state := r.conn.GetState()
	switch state {
	case connectivity.TransientFailure:
		r.connectionStatus = reporter.ConnectionStatusDisconnect
	case connectivity.Shutdown:
		r.connectionStatus = reporter.ConnectionStatusShutdown
	default:
		r.connectionStatus = reporter.ConnectionStatusConnected
	}
	return r.connectionStatus
}

func (r *gRPCReporter) initCDS(cdsWatchers []reporter.AgentConfigChangeWatcher) {
	if r.cdsClient == nil {
		return
	}

	// bind watchers
	r.cdsService.BindWatchers(cdsWatchers)

	// fetch config
	go func() {
		for {
			switch r.updateConnectionStatus() {
			case reporter.ConnectionStatusShutdown:
				break
			case reporter.ConnectionStatusDisconnect:
				time.Sleep(r.cdsInterval)
				continue
			}

			configurations, err := r.cdsClient.FetchConfigurations(context.Background(), &configuration.ConfigurationSyncRequest{
				Service: r.entity.ServiceName,
				Uuid:    r.cdsService.UUID,
			})

			if err != nil {
				r.logger.Errorf("fetch dynamic configuration error %v", err)
				time.Sleep(r.cdsInterval)
				continue
			}

			if len(configurations.GetCommands()) > 0 && configurations.GetCommands()[0].Command == "ConfigurationDiscoveryCommand" {
				command := configurations.GetCommands()[0]
				r.cdsService.HandleCommand(command)
			}

			time.Sleep(r.cdsInterval)
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
	_, err = r.managementClient.ReportInstanceProperties(metadata.NewOutgoingContext(context.Background(), r.md), &managementv3.InstanceProperties{
		Service:         r.entity.ServiceName,
		ServiceInstance: r.entity.ServiceInstanceName,
		Properties:      r.entity.Props,
	})
	return err
}

func (r *gRPCReporter) check() {
	if r.checkInterval < 0 || r.conn == nil || r.managementClient == nil {
		return
	}
	go func() {
		instancePropertiesSubmitted := false
		for {
			switch r.updateConnectionStatus() {
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

			_, err := r.managementClient.KeepAlive(metadata.NewOutgoingContext(context.Background(), r.md), &managementv3.InstancePingPkg{
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
