# Tracing, Metrics and Logging with Go Agent

All plugins in SkyWalking Go Agent are designed to provide functionality for distributed tracing, metrics, and logging data. 
For a detailed list of supported plugins, [please refer to the documentation](./support-plugins.md). 
This document aims to provide you with some configuration information for your usage. 
Please ensure that you have followed the [documentation to successfully install the SkyWalking Go Agent into your application](../setup/gobuild.md).

## Metadata Mechanism

The Go Agent would be identified by the SkyWalking backend after startup and maintain a heartbeat to keep alive.

| Name                    | Environment Key | Default Value          | Description                                                                                                                               |
|-------------------------|-----------------|------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| agent.service_name      | SW_AGENT_NAME   | Your_Application_Name  | The name of the service which showed in UI.                                                                                               |
| agent.instance_env_name |                 | SW_AGENT_INSTANCE_NAME | To obtain the environment variable key for the instance name, if it cannot be obtained, an instance name will be automatically generated. |

## Tracing

Distributed tracing is the most common form of plugin in the Go Agent, and it becomes active with each new incoming request. By default, all plugins are enabled. For a specific list of plugins, please [refer to the documentation](./support-plugins.md#tracing-plugins).

If you wish to disable a particular plugin to prevent enhancements related to that plugin, please consult the [documentation on how to disable plugins](../advanced-features/plugin-exclusion.md).

The basic configuration is as follows:

| Name                | Environment Key        | Default Value                                                | Description                                                                                                              |
|---------------------|------------------------|--------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------|
| agent.sampler       | SW_AGENT_SAMPLER       | 1                                                            | Sampling rate of tracing data, which is a floating-point value that must be between 0 and 1.                             |
| agent.ignore_suffix | SW_AGENT_IGNORE_SUFFIX | .jpg,.jpeg,.js,.css,.png,.bmp,.gif,.ico,.mp3,.mp4,.html,.svg | If the operation name of the first span is included in this set, this segment should be ignored.(multiple split by ","). |

## Metrics

The metrics plugin can dynamically monitor the execution status of the current program and aggregate the data into corresponding metrics. 
Eventually, the data is reported to the SkyWalking backend at a specified interval. For a specific list of plugins, please [refer to the documentation](./support-plugins.md#metrics-plugins).

The current configuration information is as follows:

| Name                         | Environment Key                 | Default Value  | Description                                     |
|------------------------------|---------------------------------|----------------|-------------------------------------------------|
| agent.meter.collect_interval | SW_AGENT_METER_COLLECT_INTERVAL | 20             | The interval of collecting metrics, in seconds. |

## Logging

The logging plugin in SkyWalking Go Agent are used to handle agent and application logs, as well as application log querying. They primarily consist of the following three functionalities:

1. **Agent Log Adaptation**: The plugin detects the logging framework used in the current system and integrates the agent's logs with the system's logging framework. 
2. **Distributed Tracing Enhancement**: It combines the distributed tracing information from the current request with the application logs, allowing you to have real-time visibility into all log contents related to specific requests.
3. **Log Reporting**: The plugin reports both application and agent logs to the SkyWalking backend for data retrieval and display purposes.

For more details, please [refer to the documentation to learn more detail](../advanced-features/logging-setup.md).
