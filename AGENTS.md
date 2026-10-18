# SkyWalking Go Agent Guide

Apache SkyWalking Go is a compile-time auto-instrumentation agent. The
`skywalking-go-agent` binary runs as `go build -toolexec`, rewrites the
framework packages an application uses, and links the agent runtime into the
application. Follow nearby code and tests, and keep changes focused on the
requested behavior.

## Repository layout

- The root module `github.com/apache/skywalking-go` (`agent/`, `log/`,
  `imports.go`) is what applications import with
  `_ "github.com/apache/skywalking-go"`. Its `go` directive becomes the minimum Go
  version of every application that imports it.
- `plugins/core` holds the tracing core and the plugin API (`instrument`,
  `operator`, `tracing`). It is compiled into the application.
- `plugins/<name>`: one Go module per framework plugin. Some frameworks have
  version-split plugins, such as `grpc` and `grpc-v184`, or `dubbo` and
  `dubbo-v331`.
- `tools/go-agent` builds the `skywalking-go-agent` toolexec binary. Plugins are
  registered in `tools/go-agent/instrument/plugins/register.go`. The runtime
  hook (`instrument/runtime`) and the logrus/zap integration
  (`instrument/logger`) live here too.
- `toolkit` is the public toolkit API. `test/plugins` holds the plugin scenario
  tests, `test/e2e` the end-to-end suite, and `docs/` the documentation.
- `go.work` lists the agent, toolkit, tools, plugin modules, and most test
  modules. Plugin scenario cases are built with `GOWORK=off` (see below).

## Skills

Task skills live in `.claude/skills/<name>/SKILL.md`: `build` for building,
testing, and linting, and `plugin-e2e` for plugin scenario and e2e tests.
`.agents/skills` is a symlink to the same directory, so other agents such as
Codex discover the same skills. Edit the files under `.claude/skills`.
`CLAUDE.md` is a symlink to this file. Both aliases need a checkout with
symlink support (`core.symlinks`); without it, read `AGENTS.md` and
`.claude/skills` directly.

## Go versions

- The floor is Go 1.26: the agent, toolkit, tools, and plugin modules declare
  `go 1.26.0`. Scenario and e2e fixture modules keep older `go` lines:
  `test/plugins/run.sh` rewrites each plugin case's `go` line, and the e2e
  fixtures get their Go version from their Dockerfiles. CI builds and tests with
  Go 1.26 and Go 1.27.
- Keep the floor at the oldest Go release that is still supported upstream,
  because raising the root module's `go` line forces every user application to
  that Go version.
- A dependency must not require a newer Go than the floor. Run `go get` with
  `GOTOOLCHAIN=go1.26.<latest>` so that such a dependency fails locally.

## Build, test, and lint

See `.claude/skills/build/SKILL.md` for the details. In short:

- `make build` builds `bin/skywalking-go-agent-<VERSION>-{linux,darwin,windows}-<arch>`.
- `make test` runs the tests of every module except the plugin scenarios.
- `make test-race` runs only the `TestRace*` tests, under the race detector, and
  skips the modules under `test/`.
- `make lint` runs golangci-lint, using the repository `.golangci.yml`, in every
  module except the plugin scenarios and e2e modules. Its installer is pinned to
  v2.15.0, but an existing `$(go env GOPATH)/bin/golangci-lint` is reused.
- `make check` runs `go mod tidy` in the root module only, then requires a clean
  working tree, so it also fails on edits that existed before.

## Writing plugins

These rules come from how the agent rewrites code. Ignoring them breaks user
builds, sometimes only at one end of a version range.

- **Selection.** For each compiled package, the agent uses the first registered
  instrument whose `BasePackage()` is a prefix of the package and whose
  `VersionChecker(version)` accepts the module version. The version comes from
  `vendor/modules.txt` or the module cache path. It is empty when the framework
  is replaced by a local module.
- **Copied files.** Every non-test `.go` file of the plugin package directory is
  copied into the framework package and compiled there. Files containing
  `//skywalking:nocopy` (such as `instrument.go`), `//skywalking:native`, or
  `//skywalking:ref_generate` are not copied; the agent only reads the mappings
  from the last two. Do not put helpers in those files.
- **Imports.** A copied file can only import packages that the hooked package
  imports directly, standard library packages included, in every supported
  version. The agent rewrites only two kinds of imports: the hooked package
  itself, and `github.com/apache/skywalking-go/plugins/core/{operator,log,tracing,tools,metrics,profile}`.
- **Native mirrors.** `//skywalking:native <pkg> <Type>` declares a mirror struct
  that is rewritten to the real framework type. Its field names and types must
  exist in every version the plugin accepts.
- **Matching.** For receiver methods, matching compares only the method name and
  receiver type; `WithArgType`/`WithResultType` filters apply to static
  functions only. A signature change therefore still matches, and fails at
  runtime in the interceptor's `invocation.Args()[i]` type assertions. Use
  checked type assertions.
- **Version splits.** When a framework release changes the internals the
  interceptors compile against, add a separate plugin folder rather than
  version branches in one folder:
  - give the two plugins disjoint `VersionChecker`s and the same `Name()`, so
    that plugin exclusion keeps working;
  - let the legacy plugin accept unparseable versions.
- **Plugin `go.mod`.** It requires the newest verified framework version. It does
  not affect user builds or scenario cases, but it is linked into the agent
  binary, so keep it free of vulnerable dependencies (`govulncheck` on
  `tools/go-agent`).
- **Logger code.** Code in `tools/go-agent/instrument/logger/frameworks` is
  injected into logrus/zap by AST rewriting. References to the framework package,
  such as `logrus.Fields`, are rewritten, so prefer an unnamed equivalent type
  such as `map[string]interface{}`.

### Bumping a framework version

1. Check that every hook point of the plugin exists with the same full signature
   at the lowest and highest supported version.
2. Compile a small instrumented application at both ends:
   `go build -toolexec=<agent> -a` with `GOWORK=off` and
   `replace github.com/apache/skywalking-go => <repo>`.
3. Run the scenario (`.claude/skills/plugin-e2e/SKILL.md`).
4. Update the scenario `plugin.yml`, the plugin `go.mod`,
   `docs/en/agent/support-plugins.md` (the ranges must equal the tested
   min..max), and `CHANGES.md`.

## Plugin tests

See `.claude/skills/plugin-e2e/SKILL.md` for running them. Key semantics:

- Each `test/plugins/scenarios/<name>/plugin.yml` lists `support-version`
  entries. Each entry has a Go version, framework versions, and an optional
  `excepted-file` when a version changes the reported data.
- `framework:` must be the module path exactly as written in the scenario
  `go.mod`, including a `/vN` suffix. Otherwise the per-case version is silently
  not applied. Scenarios with `framework: go` have no external framework version
  to vary, which covers standard-library, agent, and toolkit scenarios: their
  entries have no framework list, and each case is named `go<version>-native`.
- In the Docker scenario workflow, each case builds with `GOWORK=off`, with the
  agent replaced by the local code. The separate Windows workflow builds the
  `http` scenario on the host through `windows-scenarios.tpl`, which does not set
  `GOWORK=off`.
  Set `toolkit: true` when the test code imports the toolkit. Older versions can
  still be lifted by the scenario's own dependencies, so check the selected
  version (`go list -m`) when adding a lower bound.
- New scenarios must be added to the matrix in
  `.github/workflows/plugin-tests.yaml`.

## Changes, docs, and pull requests

- Add `CHANGES.md` entries at the end of the matching section of the unreleased
  version. When dropping support for library versions, list them, with the
  reason, in the version's "Unsupported Library Versions" table.
- New files that `.licenserc.yaml` does not exclude need the Apache license
  header, checked by `license-eye`. Excluded files include Markdown, `go.mod`,
  `go.sum`, `go.work`, and the protocol sources.
  The license-eye version that CI pins recurses forever on directory symlinks,
  so list every directory symlink, such as `.agents/skills`, in `paths-ignore`.
- In `.github/workflows`, `apache/*`, `actions/*`, and `github/*` actions are
  allowed. Third-party actions must be pinned to a commit SHA that is on the ASF
  allow-list (apache/infrastructure-actions).
- Pull requests are squash-merged. Keep one focused change per pull request, and
  never commit `bin/`, `test/plugins/workspace/`, `VERSION`, or `.claude/worktrees/`.

## Pitfalls

- `make build` creates a gitignored `VERSION` file once and keeps using it, so
  the binary name can drift from the commit that `test/plugins` and `test/e2e`
  expect. Delete `VERSION`, or pass `VERSION=$(git rev-parse --short HEAD)` to
  make.
- `-toolexec` builds can leave instrumented package objects in the shared Go
  build cache, which later plain builds and tests pick up. Run instrumented
  builds with a dedicated `GOCACHE`. If a plain test's stack trace shows
  `skywalking_operator...` symbols, run `go clean -cache`.
- `make test` and `make lint` find every `go.mod` in the tree, including those in
  `test/plugins/workspace/` (left by `--debug` or failed scenario runs) and in
  nested worktrees such as `.claude/worktrees/`. Remove them first, or run from a
  checkout without them.
- `make lint` reuses any `golangci-lint` already in `$(go env GOPATH)/bin`, so
  check `"$(go env GOPATH)/bin/golangci-lint" version` first.
