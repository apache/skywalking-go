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
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reporter

import (
	"fmt"
	"math/rand"
	"strings"

	"google.golang.org/grpc/resolver"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

const staticBackendScheme = "skywalking-static"

// staticBackendResolverBuilder publishes a fixed comma-separated backend list to
// gRPC. Entries are literal host:port endpoints.
type staticBackendResolverBuilder struct {
	logger      operator.LogOperator
	backends    []string
	targetValue string
	onResolved  func(addrs []string)
}

func newStaticBackendResolverBuilder(logger operator.LogOperator, backends []string,
	onResolved func(addrs []string)) (*staticBackendResolverBuilder, error) {
	if len(backends) < 2 {
		return nil, fmt.Errorf("static multi-backend resolver requires at least 2 addresses")
	}
	return &staticBackendResolverBuilder{
		logger:      logger,
		backends:    append([]string(nil), backends...),
		targetValue: fmt.Sprintf("%s:///%s", staticBackendScheme, strings.Join(backends, ",")),
		onResolved:  onResolved,
	}, nil
}

//nolint:gocritic // resolver.Builder requires resolver.Target by value.
func (b *staticBackendResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn,
	opts resolver.BuildOptions) (resolver.Resolver, error) {
	addresses := configuredAddressesAsResolverState(b.backends)
	// grpc-go 1.55 lacks pick_first.shuffleAddressList. Shuffle once per channel
	// and reuse this order for ResolveNow so healthy connections stay sticky.
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})
	if err := cc.UpdateState(resolver.State{Addresses: addresses}); err != nil {
		return nil, fmt.Errorf("static backend resolver UpdateState rejected by ClientConn: %w", err)
	}
	r := &staticBackendResolver{
		cc:         cc,
		addresses:  addresses,
		onResolved: b.onResolved,
		logger:     b.logger,
	}
	r.publish()
	return r, nil
}

func (b *staticBackendResolverBuilder) Scheme() string {
	return staticBackendScheme
}

func (b *staticBackendResolverBuilder) target() string {
	return b.targetValue
}

type staticBackendResolver struct {
	cc         resolver.ClientConn
	addresses  []resolver.Address
	onResolved func(addrs []string)
	logger     operator.LogOperator
}

func (r *staticBackendResolver) ResolveNow(opts resolver.ResolveNowOptions) {
	if err := r.cc.UpdateState(resolver.State{Addresses: r.addresses}); err != nil && r.logger != nil {
		r.logger.Errorf("static backend resolver ResolveNow UpdateState error: %v", err)
	}
}

func (r *staticBackendResolver) Close() {}

func (r *staticBackendResolver) publish() {
	if r.onResolved == nil {
		return
	}
	out := make([]string, 0, len(r.addresses))
	for _, addr := range r.addresses {
		out = append(out, addr.Addr)
	}
	r.onResolved(out)
}
