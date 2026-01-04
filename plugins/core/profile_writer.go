// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package core

import (
	"sync"
)

type ProfilingWriter struct {
	mu        sync.Mutex            // Ensures concurrent safety
	buf       []byte                // Temporary buffer for current chunk
	chunkSize int                   // Threshold size for chunking (e.g., 1MB)
	reportCh  chan<- profileRawData // Channel for sending data chunks
}

type profileRawData struct {
	data   []byte
	isLast bool
}

// NewProfilingWriter initializes a ProfilingWriter with specified chunk size and report channel
func NewProfilingWriter(chunkSize int, reportCh chan<- profileRawData) *ProfilingWriter {
	return &ProfilingWriter{
		chunkSize: chunkSize,
		reportCh:  reportCh,
		buf:       make([]byte, 0, chunkSize), // Preallocate buffer for efficiency
	}
}

// Write implements io.Writer, handles data chunking and sending
func (w *ProfilingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)

	// Send chunks when buffer reaches the threshold
	for len(w.buf) >= w.chunkSize {
		chunk := w.buf[:w.chunkSize]
		w.buf = w.buf[w.chunkSize:]

		// Send raw chunk data (business info added externally)
		w.reportCh <- profileRawData{
			data:   chunk,
			isLast: false,
		}
	}

	return len(p), nil
}

// Flush sends remaining data in the buffer
func (w *ProfilingWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.buf) > 0 {
		w.reportCh <- profileRawData{
			data:   w.buf,
			isLast: true,
		}
	} else {
		w.reportCh <- profileRawData{
			data:   nil,
			isLast: true,
		}
	}
	w.buf = nil
}
