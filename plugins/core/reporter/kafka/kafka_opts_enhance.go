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
	"time"

	logv3 "skywalking.apache.org/repo/goapi/collect/logging/v3"

	"github.com/segmentio/kafka-go"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type ReporterOptionKafka func(r *kafkaReporter)

func WithKafkaTopicSegment(topicSegment string) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.topicSegment = topicSegment
	}
}

func WithKafkaTopicMeter(topicMeter string) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.topicMeter = topicMeter
	}
}

func WithKafkaTopicLogging(topicLogging string) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.topicLogging = topicLogging
	}
}

func WithKafkaTopicManagement(topicManagement string) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.topicManagement = topicManagement
	}
}

func WithKafkaMaxSendQueueSize(kafkaMaxSendQueueSize int) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.tracingSendCh = make(chan *agentv3.SegmentObject, kafkaMaxSendQueueSize)
		r.metricsSendCh = make(chan []*agentv3.MeterData, kafkaMaxSendQueueSize)
		r.logSendCh = make(chan *logv3.LogData, kafkaMaxSendQueueSize)
	}
}

func WithKafkaBatchSize(batchSize int) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.writer.BatchSize = batchSize
	}
}

func WithKafkaBatchBytes(batchBytes int64) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.writer.BatchBytes = batchBytes
	}
}

func WithKafkaBatchTimeoutMillis(batchTimeoutMillis int) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.writer.BatchTimeout = time.Duration(batchTimeoutMillis) * time.Millisecond
	}
}

func WithKafkaAcks(acks int) ReporterOptionKafka {
	return func(r *kafkaReporter) {
		r.writer.RequiredAcks = kafka.RequiredAcks(acks)
	}
}
