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
	"time"

	"github.com/apache/skywalking-go/log"

	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	configuration "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	managementv3 "skywalking.apache.org/repo/goapi/collect/management/v3"
)

const (
	maxSendQueueSize     int32 = 30000
	defaultCheckInterval       = 20 * time.Second
	defaultCDSInterval         = 20 * time.Second
)

// NewGRPCReporter create a new reporter to send data to gRPC oap server. Only one backend address is allowed.
func NewGRPCReporter(logger log.Logger, serverAddr string, opts ...GRPCReporterOption) (Reporter, error) {
	r := &gRPCReporter{
		logger:        logger,
		sendCh:        make(chan *agentv3.SegmentObject, maxSendQueueSize),
		checkInterval: defaultCheckInterval,
		cdsInterval:   defaultCDSInterval, // cds default on
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

	conn, err := grpc.Dial(serverAddr, credsDialOption)
	if err != nil {
		return nil, err
	}
	r.conn = conn
	r.traceClient = agentv3.NewTraceSegmentReportServiceClient(r.conn)
	r.managementClient = managementv3.NewManagementServiceClient(r.conn)
	if r.cdsInterval > 0 {
		r.cdsClient = configuration.NewConfigurationDiscoveryServiceClient(r.conn)
		r.cdsService = NewConfigDiscoveryService()
	}
	return r, nil
}

type gRPCReporter struct {
	entity           *Entity
	logger           log.Logger
	sendCh           chan *agentv3.SegmentObject
	conn             *grpc.ClientConn
	traceClient      agentv3.TraceSegmentReportServiceClient
	managementClient managementv3.ManagementServiceClient
	checkInterval    time.Duration
	cdsInterval      time.Duration
	cdsService       *ConfigDiscoveryService
	cdsClient        configuration.ConfigurationDiscoveryServiceClient

	md    metadata.MD
	creds credentials.TransportCredentials

	// bootFlag is set if Boot be executed
	bootFlag bool
}

func (r *gRPCReporter) Boot(entity *Entity, cdsWatchers []AgentConfigChangeWatcher) {
	r.entity = entity
	r.initSendPipeline()
	r.check()
	r.initCDS(cdsWatchers)
	r.bootFlag = true
}

func (r *gRPCReporter) Send(spans []ReportedSpan) {
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
		// recover the panic caused by close sendCh
		if err := recover(); err != nil {
			r.logger.Errorf("reporter segment err %v", err)
		}
	}()
	select {
	case r.sendCh <- segmentObject:
	default:
		r.logger.Errorf("reach max send buffer")
	}
}

func (r *gRPCReporter) Close() {
	if r.sendCh != nil && r.bootFlag {
		close(r.sendCh)
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

func (r *gRPCReporter) initSendPipeline() {
	if r.traceClient == nil {
		return
	}
	go func() {
	StreamLoop:
		for {
			stream, err := r.traceClient.Collect(metadata.NewOutgoingContext(context.Background(), r.md))
			if err != nil {
				r.logger.Errorf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			for s := range r.sendCh {
				err = stream.Send(s)
				if err != nil {
					r.logger.Errorf("send segment error %v", err)
					r.closeStream(stream)
					continue StreamLoop
				}
			}
			r.closeStream(stream)
			r.closeGRPCConn()
			break
		}
	}()
}

func (r *gRPCReporter) initCDS(cdsWatchers []AgentConfigChangeWatcher) {
	if r.cdsClient == nil {
		return
	}

	// bind watchers
	r.cdsService.BindWatchers(cdsWatchers)

	// fetch config
	go func() {
		for {
			if r.conn.GetState() == connectivity.Shutdown {
				break
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

func (r *gRPCReporter) closeStream(stream agentv3.TraceSegmentReportService_CollectClient) {
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
			if r.conn.GetState() == connectivity.Shutdown {
				break
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
