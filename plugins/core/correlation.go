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

package core

import "sync"

// CorrelationContext is the synchronized correlation key/value storage shared
// by every span of a segment. It replaces the bare map[string]string that used
// to live on SegmentContext: that map is legitimately reachable from multiple
// goroutines (snapshot-continued child spans share it with the segment root),
// so an unsynchronized write racing the exit-span header encoding or the
// snapshot copy would crash the process with the unrecoverable
// "concurrent map iteration and map write" fatal error.
//
// SegmentContext is copied by value between the spans of a segment, therefore
// the lock cannot be embedded there; the pointer wrapper keeps a single lock
// per logical correlation store. RWMutex is the right tool here (unlike the
// span opLock): correlation is read on every propagation encode by potentially
// concurrent goroutines while writes are rare.
type CorrelationContext struct {
	mu   sync.RWMutex
	data map[string]string
}

// newCorrelationContext returns an empty store. The inner map is allocated
// lazily on the first Set: most spans never touch correlation, and the empty
// map would be one extra allocation on every segment.
func newCorrelationContext() *CorrelationContext {
	return &CorrelationContext{}
}

// newCorrelationContextFrom builds a store pre-filled with a copy of m
// (typically the correlation decoded from the inbound propagation headers).
func newCorrelationContextFrom(m map[string]string) *CorrelationContext {
	c := &CorrelationContext{data: make(map[string]string, len(m))}
	for k, v := range m {
		c.data[k] = v
	}
	return c
}

func (c *CorrelationContext) Get(key string) string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[key]
}

func (c *CorrelationContext) Set(key, value string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if value == "" {
		delete(c.data, key) // delete on a nil map is a no-op
		return
	}
	if c.data == nil {
		c.data = make(map[string]string)
	}
	c.data[key] = value
}

func (c *CorrelationContext) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// Snapshot returns a copy of the correlation data taken under the lock. The
// propagation header encoding and every other map iteration must go through it
// instead of ranging over the live map. It returns nil when empty (reading a
// nil map is safe and this avoids an allocation per exit span).
func (c *CorrelationContext) Snapshot() map[string]string {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.data) == 0 {
		return nil
	}
	cp := make(map[string]string, len(c.data))
	for k, v := range c.data {
		cp[k] = v
	}
	return cp
}

// Clone returns an independent CorrelationContext holding a copy of the data,
// used when a context snapshot crosses a goroutine boundary.
func (c *CorrelationContext) Clone() *CorrelationContext {
	if c == nil {
		return newCorrelationContext()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.data) == 0 {
		return newCorrelationContext()
	}
	cp := &CorrelationContext{data: make(map[string]string, len(c.data))}
	for k, v := range c.data {
		cp.data[k] = v
	}
	return cp
}
