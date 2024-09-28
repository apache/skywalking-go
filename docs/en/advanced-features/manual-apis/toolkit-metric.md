# Metric APIs

## Add Metrics Toolkit

toolkit/metic provides APIs to support manual reporting of metric data. Currently supports main metric types: `Counter`, `Gauge`, `Histogram`.
Add the toolkit/metric dependency to your project.

```go
import "github.com/apache/skywalking-go/toolkit/metric"
```

## Use Native Metric

### Counter

Counter are particularly useful for monitoring the rate of events in your application.
To manually build a Counter and get its value, you would typically follow these steps:

+ Create a Counter: You would create a Counter metric instance using the provided metric package functions `NewCounter(name string, opt ...MeterOpt)`. This Counter can then be used to increment its value as needed.

+ Register the Counter: After creating the Counter, it is automatically registered with the Metric Registry so that it can be tracked and exposed for monitoring purposes.

+ Increment the Counter: During the execution of your application, you would increment the Counter by `func (c *CounterRef) Inc(val float64)` method to reflect the occurrence of specific events.

+ Retrieve the Value: To get the current value of the Counter, you would access it through the `func (c *CounterRef) Get() float64`  methods.

For example:

```go
func main() {
    counter := metric.NewCounter("http_request_total")
    counter.Inc(1)
    val := counter.Get()
}	
```

### Gauge

Gauge metrics are used to represent a single numeric value that can increase or decrease, and are often used to represent metrics that can go up and down, such as memory usage, concurrency, temperature, etc.
To manually build a Gauge metric and get its value, you can follow these steps:

+ Create a Gauge metric: Use the function `NewGauge(name string, getter func() float64, opts ...MeterOpt)` provided by the metric package to create a Gauge metric instance.

+ Register Gauge Metrics: After creating a Gauge, it is automatically registered in the Metric Registry so that it can be tracked and exposed for monitoring.

+ Set Gauge Values: When creating a Gauge, we dynamically set val through a `getter func() float64` callback function type

+ Get Gauge Values: Retrieve the current value from the Gauge metric through the `(g *GaugeRef) Get() float64` method

For example:

```go
func main() {
    getCPUUsage := func() float64 {
        return 10.00
    }

    gauge := metric.NewGauge("cpu_usage_rate", getCPUUsage)
    curVal := gauge.Get()
}
```

### Histogram

Histogram metric is used to count the distribution of events. It records the frequency distribution of event values and is usually used to calculate statistics such as averages, percentiles, etc. The Histogram metric is very suitable for measuring metrics that change over time, such as request latency and response time.
To manually build a Histogram metric and get its value, you can follow these steps:

+ Create a Histogram metric: Use the `NewHistogram(name string, steps []float64, opts ...MeterOpt)` method to create a Histogram metric instance. Steps represents multiple steps in the Histogram (also called buckets in some components)

+ Register the Histogram metric: After creating the Histogram, it is automatically registered in the metric registry so that it can be tracked and exposed for monitoring.

+ Record event values to Histogram: Record event values by calling the `Observe(val float64)/ObserveWithCount(val float64, count int64)` method of the Histogram metric.

For example:

```go
func main() {
	steps := []float64{5, 10, 20, 50, 100}
	histogram := metric.NewHistogram("request_duration", steps)

	histogram.Observe(30)
	// find the value associate bucket and add specific count.
	histogram.ObserveWithCount(60, 50)
}
```

### MeterOpt

MeterOpt is a common Option for metric types. Currently, only `WithLabels` is supported to attach labels to the metric.

```go
// WithLabels Add labels for metric
func WithLabels(key, val string) MeterOpt
```

### More Information

Custom metrics may be collected by the Manual Meter API. Custom metrics collected cannot be used directly; they should be configured in the meter-analyzer-config configuration files. [see reference for details](https://skywalking.apache.org/docs/main/latest/en/setup/backend/backend-meter/#report-meter-telemetry-data)