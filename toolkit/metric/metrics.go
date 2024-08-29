package metric

type CounterRef struct{}

// Get returns the current value of the counter.
func (c *CounterRef) Get() float64 {
	return -1
}

// Inc increments the counter with value.
func (c *CounterRef) Inc(val float64) {}

type GaugeRef struct {
}

// Get returns the current value of the gauge.
func (g *GaugeRef) Get() float64 {
	return -1
}

type Histogram struct {
}

// Observe find the value associate bucket and add 1.
func (h *Histogram) Observe(val float64) {

}

// ObserveWithCount find the value associate bucket and add specific count.
func (h *Histogram) ObserveWithCount(val float64, count int64) {

}

type meterOpt struct {
}

// WithLabels Add labels for metric
func WithLabels(key, val string) meterOpt {
	return meterOpt{}
}
