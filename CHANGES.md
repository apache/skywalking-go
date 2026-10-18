Changes by Version
==================
Release Notes.

0.8.0
------------------
#### Features

* Support multi-address gRPC backends with `pick_first` failover.

#### Plugins

* Update the verified framework version ranges. Every plugin module now requires the newest version whose hook-point signatures (receiver, parameters, and results) match the last version previously tested (identical, or for dubbo-go identical through the `protocol` type aliases), and every scenario tests that version on Go 1.26 and Go 1.27. The new upper bounds are gin v1.12.0, go-restful v3.14.0, mux v1.8.1, iris v12.2.11, fasthttp v1.75.0, fiber v2.52.15, echo v4.16.0, goframe v2.10.3, dubbo-go v3.3.1, kratos v2.9.2, go-micro v4.11.2, gorm v1.31.2, gorm postgres driver v1.6.3, mongo-driver v1.17.10, go-sql-driver/mysql v1.10.1, pgx v5.11.0, go-elasticsearch v8.19.7, go-redis v9.23.0, amqp091-go v1.15.0, pulsar-client-go v0.21.0, kafka-go v0.4.51, logrus v1.10.2, and zap v1.28.0. Some lower bounds rise because those versions never actually built in their scenarios: kratos v2.6.3 is the first release whose test imports still resolve with the gRPC version the agent requires, the go-micro scenario depends on a `go-micro/examples` commit that needs go-micro v4.9.0, the gorm scenario's MySQL driver needs gorm v1.25.1, dubbo-go starts at v3.0.2 because v3.0.1 does not carry the trace context to the provider, and go-sql-driver/mysql starts at v1.5.0 because older drivers do not implement `driver.DriverContext`, so `sql.Open` never parses the DSN and the sql plugin cannot attach the peer address. dubbo-go v3.3.2 keeps the hook signatures through type aliases, but it removes the `config` package that the scenario application uses, so it is not listed yet. The kratos scenario runs in CI again.
* Support grpc v1.84.0. That release merged `processUnaryRPC` and `processStreamingRPC` into `processRPC`, removed `sendResponse`, and returns client streams wrapped in `clientStreamWrapper`, so with the previous plugin the client segment was never reported and server spans lost the RPC type tag and the `SendResponse` span. The new `plugins/grpc-v184` handles v1.84.0 and later and reports the same spans as before; `plugins/grpc` keeps handling older releases and builds whose grpc version cannot be detected. Both share the `grpc` plugin name.
* go-redis: do not create spans for the commands go-redis sends while setting up a connection. Since v9.9.0 the setup runs through the client hooks, so `HELLO` and the `AUTH` fallback exported the password in the `cache.args` and `cache.key` tags, and `SELECT`, `READONLY`, and `CLIENT SETNAME`/`SETINFO`/`MAINT_NOTIFICATIONS`/`TRACKING` appeared as cache operations of the user's trace, marked as errors against servers that do not support them. These commands are not traced when the application sends them either, since `AUTH` and `HELLO` carry credentials and the others configure the connection.
* Split the dubbo plugin by version. dubbo-go v3.3.1 moved the graceful-shutdown filter from `protocol` to `protocol/base` and `protocol/result`, so the interceptors no longer compiled into that package. `plugins/dubbo` keeps the releases before v3.3.1 and `plugins/dubbo-v331` handles v3.3.1 and later; both share the `dubbo` plugin name, and their type assertions no longer panic on unexpected arguments. With the Triple protocol of dubbo-go v3.3, the provider runs on `net/http`, so the provider entry span is created by the http plugin and named by the dubbo plugin.
* amqp: create producer spans for every publish API. Since amqp091-go v1.10.0, `Publish` and `PublishWithContext` no longer go through `PublishWithDeferredConfirmWithContext`, so they had no producer span and no context propagation; `PublishWithDeferredConfirm` is now intercepted too, with one span per message. The injected headers also reach messages published with nil `Headers`.
* zap: report logs after the pre-write hooks that zap v1.28.0 applies in `CheckedEntry.Write`, so OAP receives the same entry and fields as the configured outputs instead of values a hook redacted or rewrote.
* logrus: do not panic when formatting an entry whose `Data` is nil, which logrus v1.10.0 and later keep nil in `Entry.Dup`.

#### Documentation

* Update `docs/en/agent/support-plugins.md` with the framework version ranges verified by the plugin test scenarios, and describe the two grpc plugins.
* Update the Go versions of the published base images in `docs/en/setup/docker.md` to Go 1.26 and Go 1.27.
* Describe in `docs/en/development-and-contribution/write-plugin-testing.md` how each plugin test case pins its framework version and builds outside the repository workspace, and document the `toolkit` and `excepted-file` options.

#### Bug Fixes

* Bump vulnerable dependencies in the agent module, the reporter (`plugins/core`), and the go-agent tool. `google.golang.org/grpc` moves from v1.55.0 to v1.83.2, the lowest release that fixes every current grpc advisory, including CVE-2026-33186 (GO-2026-4762, fixed in v1.79.3), the HTTP/2 issues fixed in v1.82.1 and v1.83.1, and CVE-2026-84445 (GO-2026-6443), which was reintroduced in v1.83.0 and fixed again in v1.83.2. `golang.org/x/net` moves from v0.10.0 to v0.58.0 (required by grpc v1.83.2; this is above v0.56.0, which is the lowest release that fixes every x/net advisory except the ones fixed only in v0.60.0) and `golang.org/x/text` moves to v0.41.0. These releases require Go 1.25, so the module floor and CI move from Go 1.24 to Go 1.25 and Go 1.26. golangci-lint moves from v1.64.8 to v2.5.0, because v1.64.8 is built with Go 1.24 and refuses modules that declare `go 1.25.0`. The v2 config keeps the v1 staticcheck set (no quickfix rewrites) and leaves `grpc.Dial` in place, since it stays supported through grpc 1.x and `NewClient` would change the reporter's resolver scheme and idle behavior. grpc v1.84.0 needs the same Go 1.25 and fixes no additional advisory. `golang.org/x/net` v0.60.0 fixes the remaining HTTP/2 advisories but requires Go 1.26; the Go 1.26 floor entry below takes it.
* Fix panic when converting NoInit histograms during meter collect.
* Fix concurrent map access in RuntimeContext Get/Set/clone.
* Move the module floor from Go 1.25 to Go 1.26 and test on Go 1.26 and Go 1.27, since Go 1.25 reached end of life with the Go 1.27 release. This lets the agent module and the reporter (`plugins/core`) take `golang.org/x/net` v0.61.0, which fixes the remaining HTTP/2 advisories (GO-2026-6603, GO-2026-6610, GO-2026-6611, GO-2026-6612, GO-2026-6617; fixed in v0.60.0), and `golang.org/x/text` v0.43.0. The floor stays at Go 1.26 rather than 1.27 because the agent's `go` directive becomes the minimum for every application that imports it, and Go 1.26 is still supported upstream. The CI build matrix, plugin scenarios, e2e and Windows jobs, and the published base images (`go1.26`, `go1.27`) move with it. golangci-lint moves from v2.5.0 to v2.15.0 (built with Go 1.27) and is installed from `https://golangci-lint.run/install.sh`, because older copies of the install script pick the SBOM checksum for every release since v2.12.2. goconst keeps its previous scope (composite literals excluded), and the `grpc.Dial` deprecation exclusions also match staticcheck's new fully qualified messages.
* Build each plugin test case outside the repository `go.work`. Minimal version selection across the workspace lifted the framework to the newest version required by any plugin or scenario module, so about two thirds of the cases did not build the version they were named after. The go-elasticsearch and goframe scenarios now declare the full module path (`/v8`, `/v2`) so the per-case version is applied at all, and the cross-goroutine scenario uses the local toolkit.
* Remove every reachable third-party advisory from the `skywalking-go-agent` binary, which links the plugin modules. go-git moves to v5.19.3, go-billy to v5.9.2, and circl to v1.6.5 through the go-micro plugin, and amqp091-go moves to v1.15.0.

#### Unsupported Library Versions

Moving to Go 1.26 and testing every plugin case with its own framework version drops the following versions. No version was dropped because Go 1.26 cannot build it; the reasons are listed per row.

| Library | No longer supported | Supported in this release | Reason |
|---------|---------------------|---------------------------|--------|
| Go | 1.25 and earlier | 1.26, 1.27 | Go 1.25 reached end of life. The agent module's `go` directive is the minimum for every application that imports it. |
| grpc | before v1.83.2 | v1.83.2 to v1.84.0 | The agent module requires grpc v1.83.2, so applications cannot select an older release. |
| kratos | v2.3.1 to v2.6.2 | v2.6.3 to v2.9.2 | These releases' tests import `google.golang.org/grpc/test/grpc_testing`, which the required grpc no longer contains, so their modules do not resolve. |
| go-micro | v4.6.0 to v4.8.1 | v4.9.0 to v4.11.2 | Never tested as named. The test application needs go-micro v4.9.0 through `go-micro/examples`. |
| gorm | v1.22.0 to v1.25.0 | v1.25.1 to v1.31.2 | Never tested as named. The tested MySQL driver needs gorm v1.25.1. |
| go-sql-driver/mysql | v1.4.0 to v1.4.1 | v1.5.0 to v1.10.1 | No `driver.DriverContext`, so `sql.Open` never parses the DSN and the sql plugin creates no spans. |
| dubbo-go | v3.0.1 | v3.0.2 to v3.3.1 | Does not carry the trace context to the provider. |
| gin | v1.7.0 to v1.8.x | v1.9.0 to v1.12.0 | Listed in the documentation but not covered by any test case. |
| iris | v12.1.x | v12.2.0 to v12.2.11 | Listed in the documentation but not covered by any test case. |
| go-redis | v9.0.3 | v9.0.4 to v9.23.0 | Listed in the documentation but not covered by any test case. |

#### Issues and PR
- All issues are [here](https://github.com/apache/skywalking/milestone/262?closed=1)
- All and pull requests are [here](https://github.com/apache/skywalking-go/milestone/9?closed=1)

0.7.0
------------------
#### Features

* Support Windows plugin test.
* Support Kafka reporter.
* Add recover to goroutine to prevent unexpected panics.
* Add mutex to fix some data race. 
* Replace external `goapi` dependency with in-repo generated protocols. 
* Support pprof profiling. 
* Align the agent with the supported Go releases (retire EOL Go 1.19-1.23): publish Go 1.24, 1.25, 1.26 base images, bump the module `go.mod` floor to Go 1.24, and run the CI build, plugin, and e2e jobs on Go 1.24-1.26.
* Support managing toolkit spans across goroutines through `SpanRef`.

#### Plugins

* Support gRPC v1.81.1 with Go 1.25 and Go 1.26.

#### Documentation

#### Bug Fixes

* Fix gRPC server tracing with recent internal stream types.
* Fix plugin interceptors bypassed on Windows.
* Fix wrong tracing context switch when trace ignore plugin activated.
* Fix data race when sending trace data to reporter.
* Fix multiple data races in span lifecycle, correlation context and segment collection.
* Add recover protection for the metrics, profile and segment-transform goroutines.
* Fix the RocketMQ batch consumer span: report once with one segment reference per message (new `ExtractContext` API).
* Fix nil dereference and wrong span ownership in the RocketMQ/Pulsar async producer callbacks.
* Fix concurrent finish flags of the gRPC streaming client and the go-micro socket close.
* Fix the MongoDB command span to complete through the async API (events may fire on different goroutines).
* Fix the gorm span storage to be per-statement and the mux response writer wrapping a nil writer.
* Add recover protection for the kafka instance-check and gRPC profile-fetch goroutines.
* Fix unsynchronized consumer-tag map access in the AMQP plugin (fatal concurrent map read/write).

#### Issues and PR
- All issues are [here](https://github.com/apache/skywalking/milestone/238?closed=1)
- All and pull requests are [here](https://github.com/apache/skywalking-go/milestone/8?closed=1)

0.6.0
------------------
#### Features

* support attaching events to span in the toolkit.
* support record log in the toolkit.
* support manually report metrics in the toolkit.
* support manually set span error in the toolkit.

#### Plugins
* Support [goframev2](https://github.com/gogf/gf) goframev2.

#### Documentation
* Add docs for `AddEvent` in `Tracing APIs`
* Add `Logging APIs` document into Manual APIs.
* Add `Metric APIs` document into Manual APIs.

#### Bug Fixes
* Fix wrong docker image name and `-version` command.
* Fix redis plugin cannot work in cluster mode.
* Fix cannot find file when exec build in test/plugins.
* Fix not set span error when http status code >= 400
* Fix http plugin cannot provide peer name when optional Host is empty.
* Fix Correctly instrument newproc1 for Go 1.23+ parameter counts

#### Issues and PR
- All issues are [here](https://github.com/apache/skywalking/milestone/219?closed=1)
- All and pull requests are [here](https://github.com/apache/skywalking-go/milestone/7?closed=1)

0.5.0
------------------
* **Add go `1.23` support**.
* **Remove go `1.16`, `1.17`, and `1.18` support**.

#### Features
* Add support trace ignore.
* Enhance the observability of makefile execution.
* Update the error message if the peer address is empty when creating an exit span.
* Support enhancement go `1.23`.

#### Plugins
* Support [Pulsar](https://github.com/apache/pulsar-client-go) MQ.
* Support [Segmentio-Kafka](https://github.com/segmentio/kafka-go) MQ.
* Support http headers collection for Gin.
* Support higher versions of grpc.
* Support [go-elasticsearchv8](https://github.com/elastic/go-elasticsearch) database client framework.
* Support `http.Hijacker` interface for mux plugin.
* Support collect statements and parameters in the Gorm plugin. 

### Bug Fixes
* Fix panic error when root span finished.
* Fix when not route is found, the gin operation name is "http.Method:", example: "GET:".
* Fix got `span type is wrong` error when creating exit span with trace sampling. 

0.4.0
------------------
#### Features
* Add support ignore suffix for span name.
* Adding go `1.21` and `1.22` in docker image.

#### Plugins
* Support setting a discard type of reporter.
* Add `redis.max_args_bytes` parameter for redis plugin.
* Changing intercept point for gin, make sure interfaces could be grouped when params defined in relativePath.
* Support [RocketMQ](https://github.com/apache/rocketmq-client-go) MQ.
* Support [AMQP](https://github.com/rabbitmq/amqp091-go) MQ.
* support [Echov4](https://github.com/labstack/echo) framework.

#### Documentation

#### Bug Fixes
* Fix users can not use async api in toolkit-trace.
* Fix cannot enhance the vendor management project.
* Fix SW_AGENT_REPORTER_GRPC_MAX_SEND_QUEUE not working on metricsSendCh & logSendCh chans of gRPC reporter.
* Fix ParseVendorModule error for special case in vendor/modules.txt.
* Fix enhance method error when unknown parameter type.
* Fix wrong tracing context when trace have been sampled.
* Fix enhance param error when there are multiple params.
* Fix lost trace when multi middleware `handlerFunc` in `gin` plugin.
* Fix DBQueryContext execute error in `sql` plugin.
* Fix stack overflow as endless logs triggered.

#### Issues and PR
- All issues are [here](https://github.com/apache/skywalking/milestone/197?closed=1)
- All and pull requests are [here](https://github.com/apache/skywalking-go/milestone/4?closed=1)

0.3.0
------------------
#### Features
* Support manual tracing APIs for users.

#### Plugins
* Support [mux](https://github.com/gorilla/mux) HTTP server framework.
* Support [grpc](https://github.com/grpc/grpc-go) server and client framework.
* Support [iris](https://github.com/kataras/iris) framework.
* Support [fasthttp](https://github.com/valyala/fasthttp) framework.
* Support [fiber](https://github.com/gofiber/fiber) framework.

#### Documentation
* Add `Tracing APIs` document into `Manual APIs`.

#### Bug Fixes
* Fix Docker image not supporting the `arm64` platform.

#### Issues and PR
- All issues are [here](https://github.com/apache/skywalking/milestone/189?closed=1)
- All and pull requests are [here](https://github.com/apache/skywalking-go/milestone/3?closed=1)

0.2.0
------------------
#### Features
* Enhance the plugin rewrite ability to support `switch` and `if/else` in the plugin codes.
* Support inject the skywalking-go into project through agent.
* Support add configuration for plugin.
* Support metrics report API for plugin.
* Support report Golang runtime metrics.
* Support log reporter.
* Enhance the `logrus` logger plugin to support adapt without any settings method invoke.
* Disable sending observing data if the gRPC connection is not established for reducing the connection error log.
* Support enhance vendor management project.
* Support using base docker image to building the application.

#### Plugins
* Support [go-redis](https://github.com/redis/go-redis) v9 redis client framework.
* Support collecting [Native HTTP](https://pkg.go.dev/net/http) URI parameter on server side.
* Support [Mongo](https://github.com/mongodb/mongo-go-driver) database client framework.
* Support [Native SQL](https://pkg.go.dev/net/http) database client framework with [MySQL Driver](github.com/go-sql-driver/mysql).
* Support [Logrus](https://github.com/sirupsen/logrus) log report to the backend.
* Support [Zap](https://github.com/uber-go/zap) log report to the backend.

#### Documentation
* Combine `Supported Libraries` and `Performance Test` into `Plugins` section.
* Add `Tracing, Metrics and Logging` document into `Plugins` section.

#### Bug Fixes
* Fix throw panic when log the tracing context before agent core initialized.
* Fix plugin version matcher `tryToFindThePluginVersion` to support capital letters in module paths and versions.

#### Issues and PR
- All issues are [here](https://github.com/apache/skywalking/milestone/180?closed=1)
- All and pull requests are [here](https://github.com/apache/skywalking-go/milestone/2?closed=1)

0.1.0
------------------
#### Features
* Initialize the agent core and user import library.
* Support gRPC reporter for management, tracing protocols.
* Automatic detect the log frameworks and inject the log context.

#### Plugins
* Support [Gin](https://github.com/gin-gonic/gin) framework.
* Support [Native HTTP](https://pkg.go.dev/net/http) server and client framework.
* Support [Go Restful](https://github.com/emicklei/go-restful) v3 framework.
* Support [Dubbo](https://github.com/apache/dubbo-go) server and client framework.
* Support [Kratos](github.com/go-kratos/kratos) v2 server and client framework.
* Support [Go-Micro](https://github.com/go-micro/go-micro) v4 server and client framework.
* Support [GORM](https://github.com/go-gorm/gorm) v2 database client framework.
* Support [MySQL Driver](https://github.com/go-gorm/mysql) detection.

#### Documentation
* Initialize the documentation.

#### Issues and PR
- All issues are [here](https://github.com/apache/skywalking/milestone/176?closed=1)
- All and pull requests are [here](https://github.com/apache/skywalking-go/milestone/1?closed=1)
