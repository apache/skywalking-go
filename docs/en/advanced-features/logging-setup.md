# Logging Setup 

Logging Setup is used to integrate the Go Agent with the logging system in the current service. 
It currently supports the recognition of `Logrus` and `Zap` frameworks. If neither of these frameworks is present, it would output logs using `Std Error`.

You can learn about the configuration details through the "log" configuration item in the [default settings](../../../tools/go-agent/config/agent.default.yaml).

## Agent with Logging system

The integration of the Agent with logs can be divided into the following:

1. **Integrating Agent logs into the Service**: Integrating the logs from the Agent into the framework used by the service.
2. **Integrating Tracing information into the Service**: Integrating the information from Tracing into the service logs.

### Agent logs into the Service

Agent logs output the current running status of the Agent system, most of which are execution exceptions. 
For example, communication anomalies between the Agent and the backend service, plugin execution exceptions, etc.

Integrating Agent logs into the service's logging system can effectively help users quickly troubleshoot whether there are issues with the current Agent execution.

### Tracing information into the Service

The Agent would also enhance the existing logging system. 
When the service outputs log, if the current goroutine contains Tracing data, it would be outputted together with the current logs. 
This helps users to quickly locate the link based on the Tracing data.

#### Tracing data

The Tracing includes the following information:

1. **TraceID**: The current Trace ID. If there is no link, it outputs `Noop`.
2. **SegmentID**: The Segment ID in the current Trace. If there is no link, it outputs `Noop`.
3. **SpanID**: The Span ID currently being operated on. If there is no link, it outputs `0`.

The output format is as follows: `[${TraceID},${SegmentID},${SpanID}]`.

The following is an example of a log output when using `Zap.NewProduction`:

```
{"level":"info","ts":1683635924.717511,"caller":"gin/main.go:45","msg":"test log","SW_CTX":"[6fbe5844ee6611ed8e30acde48001122,6fbe5902ee6611ed8e30acde48001122,0]"}
```