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

package consts

const (
	GlobalTracerFieldName           = "globalSkyWalkingOperator"
	GlobalLoggerFieldName           = "globalSkyWalkingLogger"
	GlobalTracerInitNotifyFieldName = "globalSkyWalkingInitNotify"

	MetricsRegisterFieldName = "metricsRegister"
	MetricsHookFieldName     = "metricsHook"

	GlobalTracerSnapshotInterface = "skywalkingGoroutineSnapshotCreator"

	GlobalTracerSetMethodName              = "skywalking_set_global_operator"
	GlobalTracerGetMethodName              = "skywalking_get_global_operator"
	GlobalLoggerSetMethodName              = "skywalking_set_global_logger"
	GlobalLoggerGetMethodName              = "skywalking_get_global_logger"
	GlobalTracerInitAppendNotifyMethodName = "skywalking_global_init_append_notify"
	GlobalTracerInitGetNotifyMethodName    = "skywalking_global_init_get_notify"

	MetricsRegisterAppendMethodName = "skywalking_metrics_register_append"
	MetricsObtainMethodName         = "skywalking_metrics_obtain"
	MetricsHookAppendMethodName     = "skywalking_metrics_hook_append"

	CurrentGoroutineIDGetMethodName = "skywalking_get_goid"
)
