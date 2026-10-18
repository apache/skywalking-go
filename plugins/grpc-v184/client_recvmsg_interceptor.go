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

package grpcv184

import (
	"io"
	"strings"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// ClientRecvMsgInterceptor intercepts (*clientStream).RecvMsg.
//
// For non-server-streaming RPCs (unary and client-streaming), grpc < v1.84.0
// read the response message and the trailer in a single RecvMsg call. Since
// v1.84.0, RecvMsg returns right after the response message, and the caller
// (invoke for unary RPCs, clientStreamWrapper for streams) reads the trailer
// with a second RecvMsg call, which finishes the stream. To report the same
// spans as before, the span of the first call is kept open and active until
// the second call returns, and the second call does not create a span.
type ClientRecvMsgInterceptor struct {
}

// cardinalityViolation is the error grpc returns when a non-server-streaming RPC
// receives more than one response message.
const cardinalityViolation = "cardinality violation: expected <EOF> for non server-streaming RPCs, but received another message"

// clientRecvTrailer is the invocation context of the trailer read.
type clientRecvTrailer struct {
	data *contextData
}

func (h *ClientRecvMsgInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	cs := invocation.CallerInstance().(*nativeclientStream)
	method := cs.callHdr.Method
	if strings.HasPrefix(method, skywalkingService) {
		return nil
	}
	contextdata := clientStreamContextData(invocation.CallerInstance())
	if contextdata != nil && contextdata.pendingRecvSpan != nil {
		// trailer read: the span opened by the previous RecvMsg stays active,
		// so the finish interceptor works on it as it did with grpc < v1.84.0
		invocation.SetContext(&clientRecvTrailer{data: contextdata})
		return nil
	}
	if contextdata != nil {
		tracing.ContinueContext(contextdata.continueSnapShot)
		contextdata.interceptFinish.Store(true)
	}
	s, err := tracing.CreateLocalSpan(formatOperationName(method, "/Client/Response/RecvMsg"),
		tracing.WithLayer(tracing.SpanLayerRPCFramework),
		tracing.WithTag(tracing.TagURL, method),
		tracing.WithComponent(23),
	)
	if err != nil {
		return err
	}
	invocation.SetContext(s)
	return nil
}

func (h *ClientRecvMsgInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() == nil {
		return nil
	}
	err, _ := result[0].(error)
	if trailer, ok := invocation.GetContext().(*clientRecvTrailer); ok {
		span := trailer.data.pendingRecvSpan
		trailer.data.pendingRecvSpan = nil
		switch {
		case err == nil:
			// Another response message arrived instead of the trailer. grpc v1.84.0
			// returns the cardinality error from invoke or clientStreamWrapper after
			// this hook and does not finish the stream, so report it here as the
			// single RecvMsg of grpc < v1.84.0 did, and finish the stream span.
			span.Error(cardinalityViolation)
			if trailer.data.asyncSpan != nil {
				trailer.data.asyncSpan.Error(cardinalityViolation)
			}
			span.End()
			finishStreamSpan(trailer.data)
		case err != io.EOF:
			span.Error(err.Error())
			span.End()
		default:
			// io.EOF is the successful end of the trailer read, the single RecvMsg of
			// grpc < v1.84.0 returned nil in that case
			span.End()
		}
		tracing.ContinueContext(trailer.data.endSnapShot)
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	cs := invocation.CallerInstance().(*nativeclientStream)
	if err == nil && cs.desc != nil && !cs.desc.ServerStreams && keepRecvSpanUntilTrailer(invocation.CallerInstance(), span) {
		return nil
	}
	if err != nil && err != io.EOF {
		span.Error(err.Error())
	}
	if err == io.EOF {
		span.SetOperationName(formatOperationName(cs.callHdr.Method, "/Client/Response/CloseRecv"))
	}
	span.End()
	if contextdata := clientStreamContextData(invocation.CallerInstance()); contextdata != nil {
		tracing.ContinueContext(contextdata.endSnapShot)
	}
	return nil
}

// clientStreamContextData returns the context data attached to the client stream, or nil.
func clientStreamContextData(stream interface{}) *contextData {
	csEnhanced, ok := stream.(operator.EnhancedInstance)
	if !ok || csEnhanced.GetSkyWalkingDynamicField() == nil {
		return nil
	}
	contextdata, _ := csEnhanced.GetSkyWalkingDynamicField().(*contextData)
	return contextdata
}

// keepRecvSpanUntilTrailer stores the span in the client stream context data,
// so the trailer read ends it. Unary RPCs have no context data yet, so an empty
// one is attached, which leaves the finish interceptor a no-op as before.
func keepRecvSpanUntilTrailer(stream interface{}, span tracing.Span) bool {
	csEnhanced, ok := stream.(operator.EnhancedInstance)
	if !ok {
		return false
	}
	contextdata := clientStreamContextData(stream)
	if contextdata == nil {
		contextdata = &contextData{}
		csEnhanced.SetSkyWalkingDynamicField(contextdata)
	}
	contextdata.pendingRecvSpan = span
	return true
}
