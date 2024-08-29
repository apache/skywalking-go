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
