# Performance Tests

Performance testing is used to verify the impact on application performance when using SkyWalking Go.

## Test Objective

By launching both the **agent** and **non-agent** compiled applications, we subject them to the same **QPS** under stress testing, 
evaluating the CPU, memory, and network latency of the machine during the testing period.

The application has been saved and submitted to the [test/benchmark-codebase](../../../test/benchmark-codebase) directory, with the following topology:

```
traffic generator -> consumer -> provider
```

The payload(traffic) generator uses multithreading to send HTTP requests to the consumer service. 
When the consumer receives a request, it sends three requests to the provider service to obtain return data results. 
Based on these network requests, when using SkyWalking Go, the consumer service generates four Spans (1 Entry Span, 3 Exit Spans).

### Application

The application's integration with SkyWalking Go follows the same process as other applications. For more information, please [refer to the documentation](../setup/gobuild.md).

In the application, we use loops and mathematical calculations (`math.Log`) to simulate the execution of the business program. 
This consumes a certain amount of CPU usage, preventing idle processing during service stress testing and amplifying the impact of the Agent program on the business application.

### Stress Testing Service

We use the [Vegeta](https://github.com/tsenart/vegeta) service for stress testing, which launches traffic at a specified QPS to the application. 
It is based on the Go language and uses goroutines to provide a more efficient stress testing solution.

## Test Environment

A total of 4 GCP machines are launched, all instances are running on tbe 4C8G VM.

1. **traffic generator**: Used for deploying traffic to the consumer machine.
2. **consumer**: Used for deploying the consumer service.
3. **provider**: Used for deploying the provider service.
4. **skywalking**: Used for deploying the SkyWalking backend cluster, providing a standalone OAP node (in-memory H2 storage) and a UI interface.

Each service is deployed on a separate machine to ensure there is no interference with one another.

## Test Process

### Preparation Phase
The preparation phase is used to ensure that all machines and test case preparations are completed.

#### Traffic Generator

Install the [Vegeta service](https://github.com/tsenart/vegeta#install) on the stress testing instance and create the following file(`request.txt`) to simulate traffic usage.

```
GET http://${CONSUMER_IP}:8080/consumer
Sw8: 1-MWYyZDRiZjQ3YmY3MTFlYWI3OTRhY2RlNDgwMDExMjI=-MWU3YzIwNGE3YmY3MTFlYWI4NThhY2RlNDgwMDExMjI=-0-c2VydmljZQ==-aW5zdGFuY2U=-cHJvcGFnYXRpb24=-cHJvcGFnYXRpb246NTU2Ng==
```

Please replace the above `CONSUMER_IP` with the real IP address of the consumer instance.

#### Consumer and Provider

Install the skywalking-go service on the machines to be tested, and compile with and without the Agent.

Modify the machine's file limit to prevent the inability to create new connections due to excessive handles: `ulimit -n 65536`.

Start the **provider** service(without Agent) and obtain the provider machine's IP address. Please provide this address when starting the consumer machine later.

#### SkyWalking

Download the SkyWalking service, modify the SkyWalking OAP startup script to increase the memory size, preventing OAP crashes due to insufficient memory.

### Testing without Agent

1. Start the Consumer service **without the Agent version**. Please add the `provider` flag for the provider address, the format is: `http://${PROVIDER_IP}:8080/provider`.
2. Execute this command to preheat the system: `vegeta attack -duration=1m -rate=1000/s -max-workers=2000 -targets=request.txt`
3. Execute this command to perform the stress test. The command will output statistical data of the stress test when completed: 
`vegeta attack -duration=20m -rate=1000/s -max-workers=2000 -targets=request.txt | tee results.bin | vegeta report`

### Testing with Agent

The only difference in the test without the Agent is the version of the consumer that is compiled and launched.

1. Add the `SW_AGENT_REPORTER_GRPC_BACKEND_SERVICE` environment variables to the consumer service, for setting the IP address of the SkyWalking OAP service.
2. Start the Consumer service **with the Agent version**. Please add the `provider` flag for the provider address, the format is: `http://${PROVIDER_IP}:8080/provider`.
3. Execute this command to preheat the system: `vegeta attack -duration=1m -rate=1000/s -max-workers=2000 -targets=request.txt`
4. Execute this command to perform the stress test. The command will output statistical data of the stress test when completed:
   `vegeta attack -duration=20m -rate=1000/s -max-workers=2000 -targets=request.txt | tee results.bin | vegeta report`

## Test Results

In the tests, we used **1000 QPS** as a benchmark to stress test both the Consumer services with and without the Agent.

* In the **non-Agent version**, the CPU usage was around **74%**, memory usage was **2.53%**, and the average response time for a single request was **4.18ms**.
* In the **Agent-compiled version**, the CPU usage was around **81%**, memory usage was **2.61%**, and the average response time for a single request was **4.32ms**.

From these results, we can conclude that after adding the Agent, **the CPU usage increased by about 9%, memory usage experienced almost no growth, and the average response time for requests increased by approximately 0.15ms**.

Explanation, `approximately 0.15ms` is the in-band cost. The most of CPU(`extra 9%`) cost are due to the amount of out of band data being sent to the collectors from the application(consumer), which is 4000 spans/s in our test case.
