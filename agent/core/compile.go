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

import (
	//go:nolint
	_ "context"
	_ "encoding/base64"
	_ "fmt"
	_ "log"
	_ "math"
	_ "math/rand"
	_ "net"
	_ "os"
	_ "reflect"
	_ "runtime"
	_ "runtime/debug"
	_ "runtime/metrics"
	_ "runtime/pprof"
	_ "sort"
	_ "strconv"
	_ "strings"
	_ "sync"
	_ "sync/atomic"
	_ "time"
	_ "unsafe"

	//go:nolint
	_ "github.com/apache/skywalking-go/agent/core/metrics"
	_ "github.com/apache/skywalking-go/agent/core/operator"
	_ "github.com/apache/skywalking-go/agent/core/profile"
	_ "github.com/apache/skywalking-go/agent/core/tracing"
	_ "github.com/apache/skywalking-go/agent/reporter"
	_ "github.com/apache/skywalking-go/log"

	//go:nolint
	_ "github.com/google/uuid"
	_ "github.com/pkg/errors"

	//go:nolint
	_ "skywalking.apache.org/repo/goapi/collect/common/v3"
	_ "skywalking.apache.org/repo/goapi/collect/event/v3"
	_ "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	_ "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
	_ "skywalking.apache.org/repo/goapi/collect/logging/v3"
)
