# Kafka Reporter

This document describes how to configure and use the Kafka reporter in the Apache SkyWalking Go agent. The Kafka reporter provides an alternative to the default gRPC reporter, allowing you to send trace, metrics, and log data to Apache Kafka.

## Overview

The SkyWalking Go agent can be configured to report collected telemetry data (traces, metrics, logs) to a Kafka cluster. This is useful for scenarios where Kafka is already part of your infrastructure or when you prefer Kafka's buffering and scalability features for handling observability data.

**Note:** Even when the primary data reporting is set to Kafka (`reporter.type: kafka`), the CDS functionality itself still relies on gRPC communication with the SkyWalking OAP (Observability Analysis Platform). Therefore, you **must** also configure the relevant gRPC settings under `reporter.grpc` (or their corresponding environment variables like `SW_AGENT_REPORTER_GRPC_BACKEND_SERVICE`) for CDS to work correctly.


## Enabling Kafka Reporter

You can enable the Kafka reporter either through environment variables or by configuring the `agent.default.yaml` file.

**Using Environment Variables:**

Set the `SW_AGENT_REPORTER_TYPE` environment variable to `kafka`:
```bash
export SW_AGENT_REPORTER_TYPE=kafka
```

**Using `agent.default.yaml`:**

Modify the `reporter.type` setting in your `agent.default.yaml` configuration file:
```yaml
reporter:
  type: kafka # or grpc
  # ... other global reporter settings
```

## Configuration

The Kafka reporter requires specific configurations for connecting to your Kafka cluster and specifying topics for different data types. These can be set via environment variables or in the `agent.default.yaml` file. Environment variable names generally follow the pattern `SW_AGENT_REPORTER_KAFKA_OPTION_NAME_IN_UPPERCASE`.

### Core Kafka Configuration

These settings are typically found under the `reporter.kafka` section in `agent.default.yaml` or can be set using the corresponding environment variables.

*   **Broker Addresses:**
    A comma-separated list of Kafka broker addresses.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_BROKERS`
    *   YAML: `reporter.kafka.brokers`
    *   Example: `kafka1:9092,kafka2:9092`


*   **Topic for Segments:**
    The Kafka topic where trace segments will be sent.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_TOPIC_SEGMENT`
    *   YAML: `reporter.kafka.topic_segment`
    *   Example: `skywalking-segments`


*   **Topic for Metrics:**
    The Kafka topic where metrics data will be sent.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_TOPIC_METER`
    *   YAML: `reporter.kafka.topic_meter`
    *   Example: `skywalking-meters`


*   **Topic for Logs:**
    The Kafka topic where log data will be sent.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_TOPIC_LOGGING`
    *   YAML: `reporter.kafka.topic_logging`
    *   Example: `skywalking-logs`


*   **Topic for Management:** (Optional)
    The Kafka topic for management-related messages (e.g., potentially for configurations or commands in future use).
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_TOPIC_MANAGEMENT`
    *   YAML: `reporter.kafka.topic_management`
    *   Example: `skywalking-management`


### Advanced Kafka Configuration

These options allow fine-tuning of the Kafka producer behavior. They can also be configured via environment variables or in `agent.default.yaml` under `reporter.kafka`.

*   **Send Queue Size (Tracing, Metrics, Logs):**
    The maximum size of the internal queue for buffering data before sending for each data type.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_MAX_SEND_QUEUE`  
    *   YAML: `reporter.kafka.max_send_queue`
    *   Default: `5000`


*   **Batch Size:**
    The maximum number of messages batched before being sent to a partition.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_BATCH_SIZE`
    *   YAML: `reporter.kafka.batchSize`
    *   Default: `1000`


*   **Batch Bytes:**
    The maximum total bytes batched before being sent to a partition.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_BATCH_BYTES`
    *   YAML: `reporter.kafka.batchBytes`
    *   Default: `1048576` (1MB)


*   **Batch Timeout:**
    The maximum time the producer will wait before sending a batch, even if `batchSize` or `batchBytes` is not met. (e.g., "1s", "500ms").
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_BATCH_TIMEOUT`
    *   YAML: `reporter.kafka.batchTimeout`
    *   Default: `1s` (1 second)


*   **Acknowledgement (Acks):**
    The level of acknowledgement required from Kafka brokers.
    *   `0`: No acknowledgement (producer does not wait).
    *   `1`: Leader acknowledgement (producer waits for the leader to write the record).
    *   `-1`: All in-sync replicas acknowledgement.
    *   Environment Variable: `SW_AGENT_REPORTER_KAFKA_ACKS`
    *   YAML: `reporter.kafka.acks`
    *   Default: `1` (leader)


### Example `agent.default.yaml` Snippet for Kafka:

```yaml
reporter:
  # Reporter type: "grpc" or "kafka"
  type: ${SW_AGENT_REPORTER_TYPE:kafka}
  kafka:
    # Kafka broker addresses, comma separated
    brokers: ${SW_AGENT_REPORTER_KAFKA_BROKERS:127.0.0.1:9092}
    # Topic for segments data
    topic_segment: ${SW_AGENT_REPORTER_KAFKA_TOPIC_SEGMENT:skywalking-segments}
    # Topic for meters data
    topic_meter: ${SW_AGENT_REPORTER_KAFKA_TOPIC_METER:skywalking-meters}
    # Topic for logging data
    topic_logging: ${SW_AGENT_REPORTER_KAFKA_TOPIC_LOGGING:skywalking-logs}
    # Topic for management data
    topic_management: ${SW_AGENT_REPORTER_KAFKA_TOPIC_MANAGEMENT:skywalking-managements}
    # Send queue size
    max_send_queue: ${SW_AGENT_REPORTER_KAFKA_MAX_SEND_QUEUE:5000}
    # Batch size
    batch_size: ${SW_AGENT_REPORTER_KAFKA_BATCH_SIZE:1000}
    # Batch bytes
    batch_bytes: ${SW_AGENT_REPORTER_KAFKA_BATCH_BYTES:1048576}
    # Batch timeout millis
    batch_timeout_millis: ${SW_AGENT_REPORTER_KAFKA_BATCH_TIMEOUT_MILLIS:1000}
    # Acknowledge, 0: none, 1: leader, -1: all
    acks: ${SW_AGENT_REPORTER_KAFKA_ACKS:1}
```

## Data Format

The agent transforms the collected spans, metrics, and logs into the standard Apache SkyWalking data format before sending them to Kafka. The SkyWalking OAP (Observability Analysis Platform) should be configured with Kafka fetchers to consume and process this data from the specified Kafka topics. Ensure your OAP version is compatible and configured correctly to ingest data reported via Kafka.