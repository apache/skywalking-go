# Logging Setup 

Logging Setup is used to integrate the Go Agent with the logging system in the current service. 
It currently supports the recognition of `Logrus` and `Zap` frameworks. If neither of these frameworks is present, it would output logs using `Std Error`.

You can learn about the configuration details through the "log" configuration item in the [default settings](../../../tools/go-agent/config/agent.default.yaml).

## Logging Detection

Log detection means that the logging plugin would automatically detect the usage of logs in your application. 
When the log type is set to `auto`, it would choose the appropriate log based on the creation rules of different frameworks. The selection rules vary depending on the framework:

1. `Logrus`: It automatically selects the current logger when executing functions such as `logrus.New`, `logger.SetOutput`, or `logger.SetFormatter`.
2. `Zap`: It automatically selects the current logger when executing functions such as `zap.New`, `zap.NewNop`, `zap.NewProduction`, `zap.NewDevelopment`, or `zap.NewExample`.

If there are multiple different logging systems in your current application, the last-called logging system would be chosen.

## Agent with Logging system

The integration of the Agent with logs includes the two parts as following.

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

1. **ServiceName**: Current service name. 
2. **ServiceInstanceName**: Current service instance name. 
3. **TraceID**: The current Trace ID. If there is no link, it outputs `N/A`.
4. **SegmentID**: The Segment ID in the current Trace. If there is no link, it outputs `N/A`.
5. **SpanID**: The Span ID currently being operated on. If there is no link, it outputs `-1`.

The output format is as follows: `[${ServiceName},${ServiceInstanceName},${TraceID},${SegmentID},${SpanID}]`.

The following is an example of a log output when using `Zap.NewProduction`:

```
{"level":"info","ts":1683641507.052247,"caller":"gin/main.go:45","msg":"test log","SW_CTX":"[Your_ApplicationName,681e4178ee7311ed864facde48001122@192.168.50.193,6f13069eee7311ed864facde48001122,6f13070cee7311ed864facde48001122,0]"}
```