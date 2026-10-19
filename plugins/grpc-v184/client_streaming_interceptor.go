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
	"context"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/apache/skywalking-go/plugins/core/log"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

// ClientStreamingInterceptor intercepts ClientConn.NewStream, which creates the
// exit span of the stream, and newClientStreamWithParams, which creates the
// clientStream of the call. The stream that NewStream returns wraps the
// clientStream at least in a clientStreamWrapper, and user stream interceptors
// can wrap it further, so the span follows the clientStream: an OnFinish call
// option is added to it when it is created, and grpc calls it once, with the
// final status, when the clientStream finishes.
type ClientStreamingInterceptor struct {
}

// streamContextKey is the key of the contextData in the context of NewStream.
type streamContextKey struct{}

// streamState is the state of the stream span. The values are explicit because
// the agent rewrites iota in the copied files.
type streamState int

const (
	// streamCreating means that ClientConn.NewStream has not returned yet.
	streamCreating streamState = 0
	// streamFinishPending means that the clientStream finished before NewStream returned.
	streamFinishPending streamState = 1
	// streamAsync means that the span waits in async mode for the clientStream to finish.
	streamAsync streamState = 2
	// streamEnded means that the span has ended.
	streamEnded streamState = 3
)

type contextData struct {
	// asyncSpan is the span that calls PrepareAsync()
	asyncSpan tracing.Span
	// desc is the StreamDesc of the NewStream call. It tells the clientStream of
	// the call apart from the clientStream of an RPC that a stream interceptor
	// makes with the same context.
	desc interface{}

	// mu guards the fields below: grpc can finish the clientStream from its own
	// goroutine, and a stream interceptor can use the clientStream on another
	// goroutine before NewStream returns.
	mu sync.Mutex
	// continueSnapShot is the snapshot that the span has not ended,
	// When the service is in progress, it should be continued
	continueSnapShot tracing.ContextSnapshot
	// endSnapShot is the snapshot that the span has ended
	// When the service is completely finished, it should be continued
	endSnapShot tracing.ContextSnapshot
	// claimed is whether the OnFinish call option of the span is added to a
	// clientStream of the call
	claimed bool
	// claimGen identifies the clientStream creation that holds the claim. The
	// OnFinish call option of a creation that failed is ignored: grpc before
	// v1.84.0 calls it even when the creation made no attempt.
	claimGen int
	state    streamState
	// finishErr is the error that the clientStream finished with before NewStream returned
	finishErr error

	// pendingRecvSpan is the RecvMsg span of a non-server-streaming RPC that is
	// kept open until the trailer read, see ClientRecvMsgInterceptor. It is only
	// accessed from RecvMsg, which gRPC does not allow to be called concurrently.
	pendingRecvSpan tracing.Span
}

func (h *ClientStreamingInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	clientconn, ok := invocation.CallerInstance().(*nativeClientConn)
	if !ok {
		claimCreatedClientStream(invocation)
		return nil
	}
	method := invocation.Args()[2].(string)
	ctx := invocation.Args()[0].(context.Context)
	remoteAddr := clientconn.Target()
	if strings.HasPrefix(method, skywalkingService) {
		return nil
	}
	s, err := tracing.CreateExitSpan(formatOperationName(method, ""), remoteAddr, func(headerKey, headerValue string) error {
		ctx = metadata.AppendToOutgoingContext(ctx, headerKey, headerValue)
		invocation.ChangeArg(0, ctx)
		return nil
	},
		tracing.WithLayer(tracing.SpanLayerRPCFramework),
		tracing.WithTag(tracing.TagURL, method),
		tracing.WithComponent(23),
	)
	if err != nil {
		return err
	}
	s.Tag(RPCTypeTag, "Streaming")
	invocation.ChangeArg(0, context.WithValue(ctx, streamContextKey{}, &contextData{asyncSpan: s, desc: invocation.Args()[1]}))
	invocation.SetContext(s)
	return nil
}

func (h *ClientStreamingInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if _, ok := invocation.CallerInstance().(*nativeClientConn); !ok {
		attachCreatedClientStream(invocation, result...)
		return nil
	}
	if invocation.GetContext() == nil {
		return nil
	}
	span := invocation.GetContext().(tracing.Span)
	contextdata := streamContextData(invocation.Args()[0])
	if contextdata == nil || contextdata.asyncSpan != span {
		contextdata = &contextData{asyncSpan: span}
	}
	if err, ok := result[1].(error); ok && err != nil {
		// no stream is returned, so nothing else ends the span
		contextdata.end()
		span.Error(err.Error())
		span.End()
		return nil
	}
	if result[0] == nil {
		// NewStream did not return, a stream interceptor panicked
		contextdata.end()
		span.Error("the stream was not created, a stream interceptor panicked")
		span.End()
		return nil
	}
	if !contextdata.isClaimed() {
		// no clientStream of the call was created, so nothing can finish the span
		contextdata.end()
		span.End()
		return nil
	}
	span.PrepareAsync()
	continueSnapShot := tracing.CaptureContext()
	span.End()
	contextdata.setSnapshots(continueSnapShot, tracing.CaptureContext())
	if finished, err := contextdata.started(); finished {
		finishAsyncSpan(span, err)
	}
	return nil
}

// claimCreatedClientStream adds the OnFinish call option that finishes the
// stream span to the clientStream that newClientStreamWithParams creates for
// the NewStream call. The callback captures the contextData, so it does not
// depend on the clientStream, which grpc can finish from its own goroutine
// before newClientStreamWithParams returns.
func claimCreatedClientStream(invocation operator.Invocation) {
	args := invocation.Args()
	if len(args) < 2 {
		return
	}
	contextdata := streamContextData(args[0])
	if contextdata == nil || contextdata.desc != args[1] {
		return
	}
	gen, claimed := contextdata.claim()
	if !claimed {
		return
	}
	opts, _ := args[len(args)-1].([]grpc.CallOption)
	// first, so that a panicking OnFinish callback of the application, which
	// grpc does not recover, cannot keep the span from finishing
	withFinish := make([]grpc.CallOption, 0, len(opts)+1)
	withFinish = append(withFinish, grpc.OnFinish(func(err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("cannot finish the grpc stream span: %v", r)
			}
		}()
		finishCreatedStreamSpan(contextdata, gen, err)
	}))
	withFinish = append(withFinish, opts...)
	invocation.ChangeArg(len(args)-1, withFinish)
	invocation.SetContext(&createdClientStream{data: contextdata, gen: gen})
}

// createdClientStream is the invocation context of newClientStreamWithParams.
type createdClientStream struct {
	data *contextData
	gen  int
}

// attachCreatedClientStream attaches the contextData of the NewStream call to
// the clientStream that newClientStreamWithParams created for it, for the
// SendMsg, CloseSend and RecvMsg interceptors, which the application calls
// after NewStream returns. When the creation failed, the OnFinish call option
// is never called, so the next creation of the call can claim the span.
func attachCreatedClientStream(invocation operator.Invocation, result ...interface{}) {
	created, ok := invocation.GetContext().(*createdClientStream)
	if !ok || created == nil {
		return
	}
	if err, _ := result[1].(error); err != nil || result[0] == nil {
		created.data.unclaim(created.gen)
		return
	}
	if csEnhanced, ok := result[0].(operator.EnhancedInstance); ok {
		csEnhanced.SetSkyWalkingDynamicField(created.data)
	}
}

// streamContextData returns the contextData in the context of NewStream, or nil.
func streamContextData(ctx interface{}) *contextData {
	c, ok := ctx.(context.Context)
	if !ok || c == nil {
		return nil
	}
	contextdata, _ := c.Value(streamContextKey{}).(*contextData)
	return contextdata
}

// clientStreamContextData returns the contextData attached to the clientStream, or nil.
func clientStreamContextData(stream interface{}) *contextData {
	csEnhanced, ok := stream.(operator.EnhancedInstance)
	if !ok || csEnhanced.GetSkyWalkingDynamicField() == nil {
		return nil
	}
	contextdata, _ := csEnhanced.GetSkyWalkingDynamicField().(*contextData)
	return contextdata
}

// claim reserves the span for a clientStream creation, and returns the
// generation of the creation.
func (d *contextData) claim() (int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.claimed {
		return 0, false
	}
	d.claimed = true
	d.claimGen++
	return d.claimGen, true
}

// unclaim releases the span after the creation gen failed, and discards a
// finish of that creation, so that the next creation can claim the span.
func (d *contextData) unclaim(gen int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if gen != d.claimGen {
		return
	}
	d.claimed = false
	if d.state == streamFinishPending {
		d.state = streamCreating
		d.finishErr = nil
	}
}

func (d *contextData) setSnapshots(continueSnapShot, endSnapShot tracing.ContextSnapshot) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.continueSnapShot = continueSnapShot
	d.endSnapShot = endSnapShot
}

// continueSnapshot returns the snapshot to continue while the stream is in
// progress, or nil before NewStream returns.
func (d *contextData) continueSnapshot() tracing.ContextSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.continueSnapShot
}

// endSnapshot returns the snapshot to continue after a stream operation, or
// nil before NewStream returns.
func (d *contextData) endSnapshot() tracing.ContextSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.endSnapShot
}

func (d *contextData) currentGen() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.claimGen
}

func (d *contextData) isClaimed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.claimed
}

// end records that the span ended without async mode.
func (d *contextData) end() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state = streamEnded
}

// started records that NewStream returned and the span is in async mode. It
// returns true, with the error, when the clientStream has already finished, and
// the caller must finish the span.
func (d *contextData) started() (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch d.state {
	case streamCreating:
		d.state = streamAsync
	case streamFinishPending:
		d.state = streamEnded
		return true, d.finishErr
	}
	return false, nil
}

// finish records that the clientStream of the creation gen finished. It
// returns true when the caller must finish the span.
func (d *contextData) finish(gen int, err error) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.claimed || gen != d.claimGen {
		// the creation failed, or another creation holds the span
		return false
	}
	switch d.state {
	case streamCreating:
		// NewStream finishes the span when it returns
		d.state = streamFinishPending
		d.finishErr = err
	case streamAsync:
		d.state = streamEnded
		return true
	}
	return false
}

// finishStreamSpan records that the clientStream that holds the span finished
// with err, the final status of the stream, and finishes the async stream span
// with it.
func finishStreamSpan(contextdata *contextData, err error) {
	finishCreatedStreamSpan(contextdata, contextdata.currentGen(), err)
}

// finishCreatedStreamSpan is finishStreamSpan for the clientStream of the
// creation gen. When the stream finishes before ClientConn.NewStream returns,
// NewStream finishes the span.
func finishCreatedStreamSpan(contextdata *contextData, gen int, err error) {
	if contextdata.asyncSpan == nil {
		return
	}
	if contextdata.finish(gen, err) {
		finishAsyncSpan(contextdata.asyncSpan, err)
	}
}

// finishAsyncSpan finishes the span with the final status of the stream, which
// grpc reports as nil for a successful stream.
func finishAsyncSpan(span tracing.Span, err error) {
	if err != nil {
		span.Error(err.Error())
	}
	span.AsyncFinish()
}
