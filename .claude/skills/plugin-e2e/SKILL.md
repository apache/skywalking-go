---
name: plugin-e2e
description: Run SkyWalking Go plugin scenario tests and the end-to-end suite locally the way CI does, narrow a run to some framework versions, and diagnose build, dependency, runtime, and validation failures
user-invocable: true
allowed-tools: Bash, Read, Glob, Grep, Edit
---

# Run Plugin Scenario Tests and E2E Locally

Ask which scenario to run if the user did not say, or infer it from the changed
plugins. A run of several cases takes minutes, so run it in the background and
report when it ends.

Scenario names usually match the plugin directory, with these exceptions:

| Plugin | Scenario |
|---|---|
| `grpc`, `grpc-v184` | `grpc` |
| `dubbo`, `dubbo-v331` | `dubbo` |
| `runtimemetrics` | `runtime_metrics` |
| `sql` | `mysql`, `postgres` |
| `gorm` | `gorm` (MySQL driver), `gorm-postgres` (PostgreSQL driver) |
| `toolkit-activation` | `trace-activation`, `logging-activation`, `metric-activation`, `cross-goroutine` |
| logrus/zap logger (`tools/go-agent/instrument/logger`) | `logrus`, `zap` |

## Prerequisites

```bash
docker info >/dev/null && echo docker ok
docker-compose version --short    # the scenario scripts call docker-compose
yq --version                      # mikefarah yq v4, used by run.sh
go version
```

## 1. Build the test tools

Rebuild after every change to the agent, a plugin, or the runner. This builds the
linux agent for the host architecture, the runner-helper, and the validator
image. Pass `VERSION`, because a stale gitignored `VERSION` file would make the
build look for a binary name that does not exist.

```bash
make -C test/plugins build VERSION=$(git rev-parse --short HEAD)
```

## 2. Run a scenario

Write the whole output to a file outside the workspace. Validator failures and
some build errors go to the command's output only, not to the case logs.

```bash
bash test/plugins/run.sh <scenario> > /tmp/<scenario>.log 2>&1
bash test/plugins/run.sh --debug <scenario> > /tmp/<scenario>.log 2>&1   # keep workspace and actual data
```

Each `support-version` entry in `plugin.yml` produces one case per framework
version, `go<version>-<framework version>`. Scenarios with `framework: go` have no
framework list and run one `go<version>-native` case per entry. The run stops at
the first failing case. Success ends with `Scenarios: <name>, Testcases: N`.

`bash test/plugins/run.sh --clean` is a global cleanup:
- it removes all dangling Docker images and prunes unused networks and volumes;
- it deletes `test/plugins/dist` and `test/plugins/workspace`.

Run `--clean` on its own, then rebuild the test tools (step 1) before running a
scenario. It currently exits non-zero when `test/plugins/workspace` was already
absent. With a scenario name, a successful cleanup continues into that scenario,
which then fails because `dist/runner-helper` was deleted.

To run only some versions, save the scenario's `plugin.yml`, edit it, run, then
restore the saved copy. The saved copy keeps any changes made before the run.
Never commit a narrowed matrix.

```bash
cp test/plugins/scenarios/<scenario>/plugin.yml /tmp/<scenario>-plugin.yml
# edit test/plugins/scenarios/<scenario>/plugin.yml, then run the scenario
cp /tmp/<scenario>-plugin.yml test/plugins/scenarios/<scenario>/plugin.yml
```

## 3. Diagnose failures

A failed case keeps `test/plugins/workspace/<scenario>/<case>/logs/`:
- `service.log`: the service container's standard output. This includes
  `bin/startup.sh`, where the instrumented build and the application run.
- `scenarios.log` and `runner-helper.log`: the standard output of the scenario
  script, which includes the image build with `go mod tidy`, and of the
  runner-helper.

Compiler errors, panics, and validation reasons can appear only in the run output
you saved in step 2, so check that first.

| Symptom | Usual cause |
|---|---|
| `failed to solve: ... go mod tidy` | The case's framework version does not resolve with the scenario's other requirements, or a dependency needs a newer Go. Reproduce it as described in section 4. |
| `could not import X (open : no such file or directory)` | A copied plugin file imports a package that the hooked package does not import directly in that version. See the `build` skill and `AGENTS.md`. |
| `undefined: skywalking_operator...` | An identifier in copied or injected code was rewritten. |
| `panic` at startup | The application fails before serving, for example a protobuf `file "x.proto" is already registered` conflict. Give the scenario's proto file a unique name. |
| `[ERROR] <case>, validate actual data failed!` | The reported data differs from `config/excepted.yml`. The `reason:` line names the first mismatch, such as a component id, or `ActualSegmentRefIsEmptyException` when context propagation is missing. |

When a framework version legitimately reports different data, add an
expectation file and set `excepted-file: config/<file>.yml` on that version's
`support-version` entry. Do not loosen the shared expectation file.

## 4. Check what a case actually builds

`run.sh` rewrites the `framework:` requirement in the case's `go.mod`. The
Dockerfile then adds the local agent `replace` (plus the toolkit one when
`toolkit: true`) and a genproto requirement, and runs `go mod tidy` with
`GOWORK=off` inside the image. The scenario's own requirements can still lift
the framework version. The host workspace copy does not contain these edits, so
check the built image, which holds the last case that built successfully. Its name
is `<project>-service` with Compose v2 and `<project>_service` with Compose v1
(`docker-compose version --short`). `<project>` is the scenario name in lower
case with the dots removed.

```bash
docker run --rm --entrypoint go <project>-service list -m <framework module>
```

To reproduce on the host instead, use a temporary copy of the scenario:
1. set its `go` line and framework version the way `run.sh` does;
2. apply the Dockerfile's edits, with absolute paths instead of its
   container-relative ones;
3. run `go mod tidy` and `go list -m` with the case's Go version.

```bash
REPO=$(git rev-parse --show-toplevel)    # before entering the temporary copy
export GOWORK=off
go mod edit -replace=github.com/apache/skywalking-go="$REPO" \
  -require=google.golang.org/genproto@v0.0.0-20230530153820-e85fd2cbaebc
go mod edit -replace=github.com/apache/skywalking-go/toolkit="$REPO/toolkit"   # only with toolkit: true
go mod tidy && go list -m <framework module>
```

`framework:` must be the module path exactly as written in `go.mod`, including a
`/vN` suffix. Otherwise every case builds the version pinned in `go.mod`.

## 5. Adding or changing versions

1. Add the version to `plugin.yml`: the oldest Go entry for the floor, plus a
   newest-Go entry for the latest version.
2. Bump the plugin module's framework requirement in `plugins/<name>/go.mod` when
   the new version becomes the newest one.
3. Run the scenario.
4. Update `docs/en/agent/support-plugins.md` so the range is the tested min..max,
   and add a `CHANGES.md` entry at the end of the matching section.
5. List a new scenario in the `case` matrix of
   `.github/workflows/plugin-tests.yaml`.

The Docker scenarios run on ubuntu amd64 in CI. Local runs on arm64 do not
exercise amd64-only code paths in frameworks, so CI is the final check. The
separate Windows workflow builds the `http` scenario on the host with
`windows-scenarios.tpl`, without containers or `GOWORK=off`.

## 6. End-to-end suite

`test/e2e` runs the reporter cases (`grpc`, `grpc-multi-backend`, `kafka`) with
skywalking-infra-e2e. Install the CLI at the same commit as
`.github/workflows/e2e.yaml`, into an explicit directory, and run that binary from
the repository root:

```bash
make -C test/e2e build VERSION=$(git rev-parse --short HEAD)
GOBIN=/tmp/skywalking-infra-e2e/bin go install \
  github.com/apache/skywalking-infra-e2e/cmd/e2e@7e4b5b68716fdb7b79b21fa8908f9db497e1b115
/tmp/skywalking-infra-e2e/bin/e2e run -c test/e2e/case/grpc/e2e.yaml --batch-mode=true
```

## Report

List each case with pass or fail. For a failure, give the case name, where the
error appeared (the run output or a log file), the first relevant error or
`reason:` line, and the likely cause from the table.
