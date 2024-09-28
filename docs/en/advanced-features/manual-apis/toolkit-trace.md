# Tracing APIs

## Add trace Toolkit

toolkit/trace provides the APIs to enhance the trace context, such as createLocalSpan, createExitSpan, createEntrySpan, log, tag, prepareForAsync and asyncFinish. 
Add the toolkit/trace dependency to your project.

```go
import "github.com/apache/skywalking-go/toolkit/trace"
```

## Use Native Tracing

### Context Carrier

The context carrier is used to pass the context between the difference application.

When creating an Entry Span, you need to obtain the context carrier from the request. When creating an Exit Span, you need to write the context carrier into the target RPC request.

```go
type ExtractorRef func(headerKey string) (string, error)

type InjectorRef func(headerKey, headerValue string) error
```

The following demo demonstrates how to pass the Context Carrier in the Tracing API:

```go
// create a new entry span and extract the context carrier from the request
trace.CreateEntrySpan("EntrySpan", func(headerKey string) (string, error) {
    return request.Header.Get(headerKey), nil
})

// create a new exit span and inject the context carrier into the request
trace.CreateExitSpan("ExitSpan", request.Host, func(headerKey, headerValue string) error {
	request.Header.Add(headerKey, headerValue)
	return nil
})
```

### Create Span

Use `trace.CreateEntrySpan()` API to create entry span, and then use `SpanRef` to contain the reference of created span in agent kernel. 

- The first parameter is operation name of span
- the second parameter is `InjectorRef`.

```go
spanRef, err := trace.CreateEntrySpan("operationName", InjectorRef)
```

Use `trace.CreateLocalSpan()` API to create local span

- the only parameter is the operation name of span.

```go
spanRef, err := trace.CreateLocalSpan("operationName")
```

Use `trace.CreateExitSpan()` API to create exit span.

- the first parameter is the operation name of span
- the second parameter is the remote peer which means the peer address of exit operation.
- the third parameter is the `ExtractorRef`

```go
spanRef, err := trace.CreateExitSpan("operationName", "peer", ExtractorRef)
```

Use `trace.StopSpan()` API to stop current span

```go
trace.StopSpan()
```

### Add Span’s Tag and Log

Use `trace.AddLog()` to record log in span.

Use `trace.SetTag()` to add tag to span, the parameters of tag are two String which are key and value respectively.

```go
trace.AddLog(...string)

trace.SetTag("key","value")
```

### Set ComponentID

Use `trace.SetComponent()` to set the component id of the Span

- the type of parameter is int32.

```go
trace.SetComponent(ComponentID)
```

The Component ID in Span is used to identify the current component, which is declared in the [component libraries YAML](https://github.com/apache/skywalking/blob/master/oap-server/server-starter/src/main/resources/component-libraries.yml) from the OAP server side.

### Attach an event to a span

Use the `trace.AddEvent(et EventType, event string)` API to attach an event to the Span

Currently, we support the following four types of events:

```go
// DebugEventType Indicates the event type is "debug"
DebugEventType EventType = "debug"

// InfoEventType Indicates the event type is "info"
InfoEventType EventType = "info"

// WarnEventType Indicates the event type is "warn"
WarnEventType EventType = "warn"

// ErrorEventType Indicates the event type is "error"
ErrorEventType EventType = "error"
```

For example, we can add a "request timeout" event:

```go
trace.AddEvent(WarnEventType, "current request has timed out!")
```

### Async Prepare/Finish

`SpanRef` is the return value of `CreateSpan`.Use `SpanRef.PrepareAsync()` to make current span still alive until `SpanRef.AsyncFinish()` called.

* Call `PrepareAsync()`.
* Use `trace.StopSpan()` to stop span in the original goroutine.
* Propagate the `SpanRef` to any other goroutine. 
* Call `SpanRef.AsyncFinish()` in any goroutine.

### Capture/Continue Context Snapshot

1. Use `trace.CaptureContext()` to get the segment info and store it in `ContextSnapshotRef`.
2. Propagate the snapshot context to any other goroutine.
3. Use `trace.ContinueContext(snapshotRef)` to load the snapshotRef in the target goroutine.

## Reading Context

All following APIs provide **readonly** features for the tracing context from tracing system. The values are only available when the current thread is traced.

- Use `trace.GetTraceID()` API to get traceID.

  ```go
  traceID := trace.GetTraceID()
  ```

- Use `trace.GetSegmentID()` API to get segmentID.

  ```go
  segmentID := trace.GetSegmentID()
  ```

- Use `trace.GetSpanID()` API to get spanID.

  ```go
  spanID := trace.GetSpanID()
  ```

## Trace Correlation Context

Trace correlation context APIs provide a way to put custom data in tracing context. All the data in the context will be propagated with the in-wire process automatically.

Use `trace.SetCorrelation()` API to set custom data in tracing context.

```go
trace.SetCorrelation("key","value")
```

- Max element count in the correlation context is 3
- Max value length of each element is 128

CorrelationContext will remove the key when the value is empty.

Use `trace.GetCorrelation()` API to get custom data.

```go
value := trace.GetCorrelation("key")
```
