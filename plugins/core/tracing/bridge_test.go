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

package tracing

import "testing"

type ownedAdaptSpan struct {
	AdaptSpan
	owner chan struct{}
}

func (s *ownedAdaptSpan) BindOwner(owner chan struct{}) {
	s.owner = owner
}

type unownedAdaptSpan struct {
	AdaptSpan
}

func TestSpanWrapperBindOwner(t *testing.T) {
	owner := make(chan struct{})
	owned := &ownedAdaptSpan{}
	newSpanAdapter(owned).(*SpanWrapper).BindOwner(owner)
	if owned.owner != owner {
		t.Fatal("the wrapper should bind the owner to the agent span")
	}
	// a span that cannot be owned is left alone
	newSpanAdapter(&unownedAdaptSpan{}).(*SpanWrapper).BindOwner(owner)
}
