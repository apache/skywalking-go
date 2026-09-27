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
	"net"
	"strconv"
	"strings"

	"google.golang.org/grpc/resolver"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

var errNoValidBackendService = fmt.Errorf("no valid backend service addresses")

// parseBackendServiceList splits a comma-separated backend_service config into
// normalized host:port entries. Invalid entries are warned about and skipped;
// empty segments and duplicates are removed.
func parseBackendServiceList(raw string, logger operator.LogOperator) ([]string, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		host, port, err := splitBackendServiceAddress(part)
		if err != nil {
			if logger != nil {
				logger.Warnf("skipping invalid backend service address %q: %v", part, err)
			}
			continue
		}
		normalized := net.JoinHostPort(host, port)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, errNoValidBackendService
	}
	return out, nil
}

func splitBackendServiceAddress(serverAddr string) (host, port string, err error) {
	serverAddr = strings.TrimSpace(serverAddr)
	if serverAddr == "" {
		return "", "", fmt.Errorf("backend service address is empty")
	}
	host, port, err = net.SplitHostPort(serverAddr)
	if err != nil {
		return "", "", fmt.Errorf("invalid backend service address %q: %w", serverAddr, err)
	}
	if host == "" || port == "" {
		return "", "", fmt.Errorf("invalid backend service address %q", serverAddr)
	}
	for _, digit := range port {
		if digit < '0' || digit > '9' {
			return "", "", fmt.Errorf("invalid backend service port %q", port)
		}
	}
	portNumber, portErr := strconv.Atoi(port)
	if portErr != nil || portNumber < 1 || portNumber > 65535 {
		return "", "", fmt.Errorf("invalid backend service port %q", port)
	}
	return host, strconv.Itoa(portNumber), nil
}

func isIPLiteralHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	return net.ParseIP(host) != nil
}

// configuredAddressesAsResolverState builds resolver addresses with ServerName
// set from each endpoint's host so TLS SNI / certificate verification follows
// the dialed backend rather than a fixed channel authority.
func configuredAddressesAsResolverState(backends []string) []resolver.Address {
	addresses := make([]resolver.Address, 0, len(backends))
	for _, cfg := range backends {
		host, _, err := net.SplitHostPort(cfg)
		if err != nil {
			addresses = append(addresses, resolver.Address{Addr: cfg})
			continue
		}
		addresses = append(addresses, resolver.Address{Addr: cfg, ServerName: host})
	}
	return addresses
}
