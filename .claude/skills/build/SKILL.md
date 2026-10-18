---
name: build
description: Build the SkyWalking Go agent binary, run unit/race tests and lint, check the Go 1.26 floor, and compile an instrumented application to verify plugin code at both ends of a framework version range
user-invocable: true
allowed-tools: Bash, Read, Glob, Grep
---

# Build and Verify the SkyWalking Go Agent

Infer what the user wants from the request or recent changes, then run the
matching steps. Run the smallest relevant step first, and report each command
with its result.

## Prerequisites

```bash
go version                      # Go 1.26 or newer; CI uses 1.26 and 1.27
git rev-parse --short HEAD      # the build version
ls VERSION 2>/dev/null && cat VERSION   # see "Agent binary"
```

## Agent binary

`make build` creates the gitignored `VERSION` file the first time and keeps using
it, so the binary name can drift from the current commit. Pass the version
explicitly:

```bash
make -C tools/go-agent build VERSION=$(git rev-parse --short HEAD)   # linux, darwin, windows
make -C tools/go-agent darwin VERSION=$(git rev-parse --short HEAD)  # one platform
ls bin/skywalking-go-agent-*
```

## Tests

`make test` runs `go test` in every module it finds, except the plugin scenarios.
That includes leftover scenario copies and nested worktrees, so remove those
first:

```bash
rm -rf test/plugins/workspace
make test             # every module except the plugin scenarios
make test-race        # only TestRace*, under the race detector, outside test/
(cd plugins/<name> && go test ./...)   # one module
```

Check the floor: a dependency that needs a newer Go fails here instead of in CI.

```bash
GOTOOLCHAIN=go1.26.<latest patch> go test ./...
```

## Lint and consistency

`make lint` uses `$(go env GOPATH)/bin/golangci-lint`. It installs v2.15.0, the
CI version, only when that binary is missing, and otherwise reuses whatever
version is installed. Check that exact binary, not the one on `PATH`. The install
URL matters: older copies of the install script fail the checksum check for
releases with SBOMs.

```bash
LINT="$(go env GOPATH)/bin/golangci-lint"
"$LINT" version
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.15.0
make lint             # skips the plugin scenario and e2e modules
(cd <module> && "$LINT" run --config "$(git rev-parse --show-toplevel)/.golangci.yml" ./...)
make check            # root go mod tidy, then requires a clean working tree
```

`make check` fails on any uncommitted change, including ones that existed before
it ran. `license-eye header check` (apache/skywalking-eyes) verifies the license
headers of the files that `.licenserc.yaml` does not exclude.

## Instrumented compile check

Run this check after changing any code that the agent copies or injects into a
framework: plugin interceptors, native mirrors, or the logger and runtime
templates. Unit tests do not cover it. Copied files compile inside the framework
package, where only that package's direct imports are available.

Two rules for the commands below:
- Build the agent inside the repository workspace, because `tools/go-agent`
  resolves the plugin modules through `go.work`. Build the application outside
  it.
- Use a dedicated `GOCACHE`. Instrumented builds otherwise leave instrumented
  package objects in the shared build cache, which later plain builds reuse.

```bash
set -euo pipefail     # stop before inspecting or running a stale app after a failed build
REPO=$(git rev-parse --show-toplevel)
WORK=$(mktemp -d)
(cd "$REPO/tools/go-agent" &&
  GOWORK="$REPO/go.work" GOFLAGS= GOCACHE="$WORK/gocache" go build -o "$WORK/agent" ./cmd)
export GOWORK=off GOFLAGS=-mod=mod GOCACHE="$WORK/gocache"

mkdir "$WORK/app" && cd "$WORK/app"
cat > main.go <<'EOF'
package main

import (
	_ "github.com/apache/skywalking-go"
	// import the framework and call the hooked API, so the hook is compiled in
)

func main() {}
EOF
go mod init app
go mod edit -replace=github.com/apache/skywalking-go="$REPO" \
  -require=google.golang.org/genproto@v0.0.0-20230530153820-e85fd2cbaebc
go get <framework module>@<version>
go mod tidy
go list -m <framework module>                 # the version actually selected
go build -toolexec="$WORK/agent" -a -work -o app . 2>&1 | tee build.log
go tool nm app | grep -F '<hooked package path>' | grep -F '<expected generated symbol>'
SW_AGENT_REPORTER_GRPC_BACKEND_SERVICE=127.0.0.1:1 ./app
```

Verify the specific generated code of the change, because the agent itself links
gRPC and a generic match proves nothing:
- for a framework plugin, the interceptor of the hooked package, such as
  `(*skywalking_operatorTypeRestfulAddFilterInterceptor).BeforeInvoke`;
- for logger or runtime changes, which generate helpers rather than
  interceptors, their specific generated symbols (for example
  `skywalking_operatorStaticMethodLogrus...` or the runtime `_skywalking_*`
  functions), or the rewritten source in the `WORK=` directory that
  `build.log` prints.

Repeat at the lowest and highest version of the plugin's range. For a version
split, also check the boundary versions on both sides.

Common failures:
- `could not import X (open : no such file or directory)`: a copied file imports
  a package that the hooked package does not import directly in that version.
- `undefined: skywalking_operator...`: an identifier in a copied or injected file
  was rewritten. In logger templates, prefer unnamed types to framework types.

## Report

Summarize the commands you ran and their pass or fail results. For failures,
include the first compiler or test error, with its file and line.
