# Tracing Plugins
The version ranges below are verified by the plugin test scenarios. Each test case builds the application with exactly the listed framework version on Go 1.26, and the newest version is also tested on Go 1.27.

The following plugins provide the distributed tracing capability, and the OAP backend would analyze the topology and
metrics based on the tracing data.

* HTTP Server
  * `gin`: [Gin](https://github.com/gin-gonic/gin) tested v1.9.0 to v1.12.0.
  * `http`: [Native HTTP](https://pkg.go.dev/net/http) tested go v1.26 to go v1.27.
  * `go-restfulv3`: [Go-Restful](https://github.com/emicklei/go-restful) tested v3.7.1 to v3.14.0.
  * `mux`: [Mux](https://github.com/gorilla/mux) tested v1.7.0 to v1.8.1.
  * `iris`: [Iris](https://github.com/kataras/iris) tested v12.2.0 to v12.2.11.
  * `fasthttp`: [FastHttp](https://github.com/valyala/fasthttp) tested v1.10.0 to v1.75.0.
  * `fiber`: [Fiber](https://github.com/gofiber/fiber) tested v2.49.0 to v2.52.15.
  * `echov4`: [Echov4](https://github.com/labstack/echo) tested v4.0.0 to v4.16.0.
  * `goframe`: [GoFrame](https://github.com/gogf/gf/) tested v2.6.0 to v2.10.3.
* HTTP Client
  * `http`: [Native HTTP](https://pkg.go.dev/net/http) tested go v1.26 to go v1.27.
  * `fasthttp`: [FastHttp](https://github.com/valyala/fasthttp) tested v1.10.0 to v1.75.0.
* RPC Frameworks
  * `dubbo`: [Dubbo](https://github.com/apache/dubbo-go) tested v3.0.2 to v3.3.1. Releases before v3.3.1 are instrumented by `plugins/dubbo`, and v3.3.1 and later by `plugins/dubbo-v331`. Both share the `dubbo` name for plugin exclusion. With the Triple protocol of dubbo-go v3.3, the provider runs on `net/http`, so its entry span carries the Go HTTP server component and tags, named after the Dubbo method.
  * `kratosv2`: [Kratos](https://github.com/go-kratos/kratos) tested v2.6.3 to v2.9.2.
  * `microv4`: [Go-Micro](https://github.com/go-micro/go-micro) tested v4.9.0 to v4.11.2.
  * `grpc` : [gRPC](https://github.com/grpc/grpc-go) tested v1.83.2 to v1.84.0. The agent module requires v1.83.2 or newer (the reporter links this library). Releases before v1.84.0 are instrumented by `plugins/grpc`, and v1.84.0 and later by `plugins/grpc-v184`. Both report the same spans and share the `grpc` name for plugin exclusion.
* Database Client
  * `gorm`: [GORM](https://github.com/go-gorm/gorm) tested v1.25.1 to v1.31.2.
    * [MySQL Driver](https://github.com/go-gorm/mysql)
    * [PostgreSQL Driver](https://github.com/go-gorm/postgres) tested v1.5.2 to v1.6.3.
  * `mongo`: [Mongo](https://github.com/mongodb/mongo-go-driver) tested v1.10.5 to v1.17.10.
  * `sql`: [Native SQL](https://pkg.go.dev/database/sql) tested go v1.26 to go v1.27.
    * [MySQL Driver](https://github.com/go-sql-driver/mysql) tested v1.5.0 to v1.10.1.
    * [pgx/v5 stdlib](https://github.com/jackc/pgx/tree/master/stdlib) tested v5.5.5 to v5.11.0.
  * `go-elasticsearchv8`: [go-elasticsearch](https://github.com/elastic/go-elasticsearch) tested v8.10.0 to v8.19.7.
* Cache Client
  * `go-redisv9`: [go-redis](https://github.com/redis/go-redis) tested v9.0.4 to v9.23.0.
* MQ Client
  * `rocketMQ`: [rocketmq-client-go](https://github.com/apache/rocketmq-client-go) tested v2.1.2.
  * `amqp`: [AMQP](https://github.com/rabbitmq/amqp091-go) tested v1.9.0 to v1.15.0.
  * `pulsar`: [pulsar-client-go](https://github.com/apache/pulsar-client-go) tested v0.12.0 to v0.21.0.
  * `segmentio-kafka`: [segmentio-kafka](https://github.com/segmentio/kafka-go) tested v0.4.47 to v0.4.51.

# Metrics Plugins
The meter plugin provides the advanced metrics collections.

* `runtimemetrics`: [Native Runtime Metrics](https://pkg.go.dev/runtime/metrics) tested go v1.26 to go v1.27.

# Logging Plugins
The logging plugin provides the advanced logging collections.

* `logrus`: [Logrus](https://github.com/sirupsen/logrus) tested v1.8.2 to v1.10.2.
* `zap`: [Zap](http://go.uber.org/zap) tested v1.17.0 to v1.28.0.
