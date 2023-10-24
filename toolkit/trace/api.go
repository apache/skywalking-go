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

package trace

func CreateEntrySpan(operationName string, extractor ExtractorRef) (s SpanRef, err error) {
	return nil, err
}

// nolint
func CreateExitSpan(operationName string, peer string, injector InjectorRef) (s SpanRef, err error) {
	return nil, err
}

func CreateLocalSpan(operationName string) (s SpanRef, err error) {
	return nil, err
}

func StopSpan() {
}

func CaptureContext() ContextSnapshotRef {
	return nil
}

func ContinueContext(ctx ContextSnapshotRef) {
}

func SetOperationName(string) {
}

func GetTraceID() string {
	return ""
}

func GetSegmentID() string {
	return ""
}

func GetSpanID() int32 {
	return -1
}

// nolint
func SetTag(key string, value string) {
}

func AddLog(...string) {
}

func PrepareAsync() {
}

func AsyncFinish() {
}

func GetCorrelation(key string) string {
	return ""
}

// nolint
func SetCorrelation(key string, value string) {
}

func SetComponent(componentID int32) {
}
