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

package kafka

import (
	"context"
	"github.com/apache/skywalking-go/plugins/core/profile"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
	"google.golang.org/protobuf/proto"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	logv3 "skywalking.apache.org/repo/goapi/collect/logging/v3"
	managementv3 "skywalking.apache.org/repo/goapi/collect/management/v3"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
)

const (
	kafkaMaxSendQueueSize int32 = 30000
	topicKeyRegister            = "register-"
)

var internalReporterContextKey = context.Background()

type kafkaReporter struct {
	entity           *reporter.Entity
	logger           operator.LogOperator
	writer           *kafka.Writer
	brokerAddr       []string
	tracingSendCh    chan *agentv3.SegmentObject
	metricsSendCh    chan []*agentv3.MeterData
	logSendCh        chan *logv3.LogData
	bootFlag         bool
	checkInterval    time.Duration
	connectionStatus reporter.ConnectionStatus
	topicSegment     string
	topicMeter       string
	topicLogging     string
	topicManagement  string
	transform        *reporter.Transform
	cdsManager       *reporter.CDSManager
}

func NewKafkaReporter(logger operator.LogOperator,
	brokers string,
	checkInterval time.Duration,
	cdsManager *reporter.CDSManager,
	opts ...ReporterOptionKafka,
) (reporter.Reporter, error) {
	r := &kafkaReporter{
		logger:           logger,
		tracingSendCh:    make(chan *agentv3.SegmentObject, kafkaMaxSendQueueSize),
		metricsSendCh:    make(chan []*agentv3.MeterData, kafkaMaxSendQueueSize),
		logSendCh:        make(chan *logv3.LogData, kafkaMaxSendQueueSize),
		checkInterval:    checkInterval,
		connectionStatus: reporter.ConnectionStatusDisconnect,
		cdsManager:       cdsManager,
	}

	r.brokerAddr = strings.Split(brokers, ",")
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(r.brokerAddr...),
		Balancer:               &kafka.RoundRobin{},
		MaxAttempts:            10,
		BatchSize:              1000,
		BatchBytes:             1048576,
		BatchTimeout:           1000 * time.Millisecond,
		RequiredAcks:           kafka.RequireOne,
		Async:                  false,
		Compression:            compress.None,
		ErrorLogger:            kafka.LoggerFunc(logger.Errorf),
		AllowAutoTopicCreation: true,
	}
	r.writer = writer
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

func (r *kafkaReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
	r.entity = entity
	r.transform = reporter.NewTransform(entity)
	r.updateConnectionStatus()
	r.initSendPipeline()
	r.check()
	r.cdsManager.InitCDS(entity, cdsWatchers)
	r.bootFlag = true
}

func (r *kafkaReporter) updateConnectionStatus() {
	if r.checkKafkaConnection() {
		r.connectionStatus = reporter.ConnectionStatusConnected
	} else {
		r.connectionStatus = reporter.ConnectionStatusDisconnect
	}
}

func (r *kafkaReporter) checkKafkaConnection() bool {
	firstAddr := r.brokerAddr[0]
	conn, err := kafka.Dial("tcp", firstAddr)
	if err != nil {
		r.logger.Errorf("kafka connection error %v", err)
		return false
	}
	defer func() {
		if err = conn.Close(); err != nil {
			r.logger.Errorf("kafka connection close error %v", err)
		}
	}()
	_, err = conn.Brokers()
	if err != nil {
		r.logger.Errorf("kafka connection error %v", err)
		return false
	}
	return true
}

func (r *kafkaReporter) initSendPipeline() {
	go r.tracingSendLoop()
	go r.metricsSendLoop()
	go r.logSendLoop()
}

func (r *kafkaReporter) tracingSendLoop() {
	consecutiveErrors := 0
	logFrequency := 30
	for s := range r.tracingSendCh {
		payload, err := proto.Marshal(s)
		if err != nil {
			r.logger.Errorf("marshal segment error %v", err)
			continue
		}
		ctx := context.WithValue(context.Background(), internalReporterContextKey, true)
		err = r.writer.WriteMessages(ctx, kafka.Message{
			Topic: r.topicSegment,
			Key:   []byte(s.GetTraceSegmentId()),
			Value: payload,
		})
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors == 1 || consecutiveErrors%logFrequency == 0 {
				r.logger.Errorf("send segment to kafka error %v (errors: %d)", err, consecutiveErrors)
			}
			continue
		} else if consecutiveErrors > 0 {
			consecutiveErrors = 0
		}
	}
}

func (r *kafkaReporter) metricsSendLoop() {
	consecutiveErrors := 0
	logFrequency := 30
	for s := range r.metricsSendCh {
		payload, err := proto.Marshal(&agentv3.MeterDataCollection{
			MeterData: s,
		})
		if err != nil {
			r.logger.Errorf("marshal metrics error %v", err)
			continue
		}
		ctx := context.WithValue(context.Background(), internalReporterContextKey, true)
		err = r.writer.WriteMessages(ctx, kafka.Message{
			Topic: r.topicMeter,
			Key:   []byte(r.entity.ServiceInstanceName),
			Value: payload,
		})
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors == 1 || consecutiveErrors%logFrequency == 0 {
				r.logger.Errorf("send metrics to kafka error %v (errors: %d)", err, consecutiveErrors)
			}
			continue
		} else if consecutiveErrors > 0 {
			consecutiveErrors = 0
		}
	}
}

func (r *kafkaReporter) logSendLoop() {
	consecutiveErrors := 0
	logFrequency := 30
	for s := range r.logSendCh {
		payload, err := proto.Marshal(s)
		if err != nil {
			r.logger.Errorf("marshal log error %v", err)
			continue
		}
		ctx := context.WithValue(context.Background(), internalReporterContextKey, true)
		err = r.writer.WriteMessages(ctx, kafka.Message{
			Topic: r.topicLogging,
			Key:   []byte(s.Service),
			Value: payload,
		})
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors == 1 || consecutiveErrors%logFrequency == 0 {
				r.logger.Errorf("send log to kafka error %v (errors: %d)", err, consecutiveErrors)
			}
			continue
		} else if consecutiveErrors > 0 {
			consecutiveErrors = 0
		}
	}
}

func (r *kafkaReporter) SendTracing(spans []reporter.ReportedSpan) {
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

func (r *kafkaReporter) SendMetrics(metrics []reporter.ReportedMeter) {
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

func (r *kafkaReporter) SendLog(log *logv3.LogData) {
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

func (r *kafkaReporter) check() {
	if r.checkInterval < 0 || r.writer == nil {
		return
	}
	go func() {
		// initialDelay
		time.Sleep(r.checkInterval)
		instancePropertiesSubmitted := false
		for {
			if !instancePropertiesSubmitted {
				instanceProperties := &managementv3.InstanceProperties{
					Service:         r.entity.ServiceName,
					ServiceInstance: r.entity.ServiceInstanceName,
					Properties:      r.entity.Props,
				}
				payload, err := proto.Marshal(instanceProperties)
				if err != nil {
					r.logger.Errorf("marshal instance properties error %v", err)
					time.Sleep(r.checkInterval)
					continue
				}
				ctx := context.WithValue(context.Background(), internalReporterContextKey, true)
				err = r.writer.WriteMessages(ctx, kafka.Message{
					Topic: r.topicManagement,
					Key:   []byte(topicKeyRegister + r.entity.ServiceInstanceName),
					Value: payload,
				})
				if err != nil {
					r.logger.Errorf("send instance properties to kafka error %v", err)
					time.Sleep(r.checkInterval)
					continue
				}
				instancePropertiesSubmitted = true
			}

			ping := &managementv3.InstancePingPkg{
				Service:         r.entity.ServiceName,
				ServiceInstance: r.entity.ServiceInstanceName,
			}
			payload, err := proto.Marshal(ping)
			if err != nil {
				r.logger.Errorf("marshal instance ping error %v", err)
				time.Sleep(r.checkInterval)
				continue
			}
			ctx := context.WithValue(context.Background(), internalReporterContextKey, true)
			err = r.writer.WriteMessages(ctx, kafka.Message{
				Topic: r.topicManagement,
				Key:   []byte(r.entity.ServiceInstanceName),
				Value: payload,
			})
			if err != nil {
				r.logger.Errorf("send instance ping to kafka error %v", err)
			}
			time.Sleep(r.checkInterval)
		}
	}()
}

func (r *kafkaReporter) ConnectionStatus() reporter.ConnectionStatus {
	return r.connectionStatus
}

func (r *kafkaReporter) Close() {
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
		if err := r.writer.Close(); err != nil {
			r.logger.Errorf("close kafka writer failed, err: %v", err)
		}
	}
}
func (r *kafkaReporter) AddProfileManager(p *profile.ProfileManager) {}

//func (r *kafkaReporter) Profiling(traceId string, endPoint string)                                {}
//func (r *kafkaReporter) EndProfiling(segmentID string)                                            {}
//func (r *kafkaReporter) AddSpanIdToProfile(segmentId string, spanId int32)                        {}
//func (r *kafkaReporter) CheckProfileValue(segmentID string, spanId int32, start int64, end int64) {}
