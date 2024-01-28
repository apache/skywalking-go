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
	"fmt"
	defLog "log"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"

	logv3 "skywalking.apache.org/repo/goapi/collect/logging/v3"
)

// nolint
const defaultLogPrefix = "skywalking-go "

type CorrelationConfig struct {
	MaxKeyCount  int
	MaxValueSize int
}

type Tracer struct {
	ServiceEntity *reporter.Entity
	Reporter      reporter.Reporter
	// 0 not init 1 init
	initFlag    int32
	Sampler     Sampler
	Log         *LogWrapper
	correlation *CorrelationConfig
	cdsWatchers []reporter.AgentConfigChangeWatcher
	// for plugin tools
	tools *TracerTools
	// for all metrics
	meterMap              *sync.Map
	meterCollectListeners []func()
	ignoreSuffix          []string
}

func (t *Tracer) Init(entity *reporter.Entity, rep reporter.Reporter, samp Sampler, logger operator.LogOperator,
	meterCollectSecond int, correlation *CorrelationConfig, ignoreSuffixStr string) error {
	t.ServiceEntity = entity
	t.Reporter = rep
	t.Sampler = samp
	if logger != nil && !reflect.ValueOf(logger).IsZero() {
		t.Log.ChangeLogger(logger)
	}
	t.Reporter.Boot(entity, t.cdsWatchers)
	t.initFlag = 1
	t.initMetricsCollect(meterCollectSecond)
	t.correlation = correlation
	t.ignoreSuffix = strings.Split(ignoreSuffixStr, ",")
	// notify the tracer been init success
	if len(GetInitNotify()) > 0 {
		for _, fun := range GetInitNotify() {
			fun()
		}
	}
	return nil
}

func (t *Tracer) Entity() interface{} {
	return t.ServiceEntity
}

func (t *Tracer) Tools() interface{} {
	return t.tools
}

func NewEntity(service, instanceEnvName string) *reporter.Entity {
	instanceName := os.Getenv(instanceEnvName)
	if instanceName == "" {
		id, err := UUID()
		if err != nil {
			panic(fmt.Sprintf("generate UUID failure: %v", err))
		}
		instanceName = id + "@" + IPV4()
	}
	propResult := buildOSInfo()
	return &reporter.Entity{
		ServiceName:         service,
		ServiceInstanceName: instanceName,
		Props:               propResult,
	}
}

// create tracer when init the agent core
// nolint
func newTracer() *Tracer {
	return &Tracer{
		initFlag:    0,
		Reporter:    &emptyReporter{},
		Sampler:     NewConstSampler(false),
		Log:         &LogWrapper{newDefaultLogger()},
		cdsWatchers: make([]reporter.AgentConfigChangeWatcher, 0),
		tools:       NewTracerTools(),

		meterMap: &sync.Map{},
	}
}

func (t *Tracer) InitSuccess() bool {
	return t.initFlag == 1
}

func (t *Tracer) ChangeLogger(logger interface{}) {
	t.Log.ChangeLogger(logger.(operator.LogOperator))
}

// nolint
type emptyReporter struct{}

// nolint
func (e *emptyReporter) Boot(entity *reporter.Entity, cdsWatchers []reporter.AgentConfigChangeWatcher) {
}

// nolint
func (e *emptyReporter) SendTracing(spans []reporter.ReportedSpan) {
}

// nolint
func (e *emptyReporter) SendMetrics(metrics []reporter.ReportedMeter) {
}

// nolint
func (e *emptyReporter) SendLog(log *logv3.LogData) {
}

func (e *emptyReporter) ConnectionStatus() reporter.ConnectionStatus {
	return reporter.ConnectionStatusDisconnect
}

// nolint
func (e *emptyReporter) Close() {
}

type LogWrapper struct {
	Logger operator.LogOperator
}

func (l *LogWrapper) ChangeLogger(logger operator.LogOperator) {
	l.Logger = logger
}

func (l *LogWrapper) WithField(key string, value interface{}) interface{} {
	return l.Logger.WithField(key, value)
}

func (l *LogWrapper) Info(args ...interface{}) {
	l.Logger.Info(args...)
}

func (l *LogWrapper) Infof(format string, args ...interface{}) {
	l.Logger.Infof(format, args...)
}

func (l *LogWrapper) Warn(args ...interface{}) {
	l.Logger.Warn(args...)
}

func (l *LogWrapper) Warnf(format string, args ...interface{}) {
	l.Logger.Warnf(format, args...)
}

func (l *LogWrapper) Error(args ...interface{}) {
	l.Logger.Error(args...)
}

func (l *LogWrapper) Errorf(format string, args ...interface{}) {
	l.Logger.Errorf(format, args...)
}

// nolint
type defaultLogger struct {
	log *defLog.Logger
}

// nolint
func newDefaultLogger() *defaultLogger {
	return &defaultLogger{
		log: defLog.New(os.Stderr, defaultLogPrefix, defLog.LstdFlags),
	}
}

func (d *defaultLogger) WithField(key string, value interface{}) interface{} {
	return d
}

// nolint
func (d *defaultLogger) Info(args ...interface{}) {
	d.log.Print(args...)
}

// nolint
func (d *defaultLogger) Infof(format string, args ...interface{}) {
	d.log.Printf(format, args...)
}

// nolint
func (d *defaultLogger) Warn(args ...interface{}) {
	d.log.Print(args...)
}

// nolint
func (d *defaultLogger) Warnf(format string, args ...interface{}) {
	d.log.Printf(format, args...)
}

// nolint
func (d *defaultLogger) Error(args ...interface{}) {
	d.log.Print(args...)
}

// nolint
func (d *defaultLogger) Errorf(format string, args ...interface{}) {
	d.log.Printf(format, args...)
}
