package metric

// NewCounter creates a new counter metrics.
func NewCounter(name string, opt ...meterOpt) *CounterRef {
	return &CounterRef{}
}

// NewGauge creates a new gauge metrics.
func NewGauge(name string, watcher func() float64, opts ...meterOpt) *GaugeRef {
	return &GaugeRef{}
}

// NewHistogram creates a new histogram metrics.
func NewHistogram(name string, steps []float64, opts ...meterOpt) *Histogram {
	return &Histogram{}
}
