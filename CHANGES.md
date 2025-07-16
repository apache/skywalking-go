Changes by Version
==================
Release Notes.

0.7.0
------------------
#### Features

* Support Windows plugin test.
* Add mutex to fix some data race. 

#### Plugins

#### Documentation

#### Bug Fixes

* Fix plugin interceptors bypassed on Windows.

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
