# Tracing Plugins
The following plugins provide the distributed tracing capability, and the OAP backend would analyze the topology and
metrics based on the tracing data.

* HTTP Server
  * `gin`: [Gin](https://github.com/gin-gonic/gin) tested v1.7.0 to v1.9.0.
  * `http`: [Native HTTP](https://pkg.go.dev/net/http) tested go v1.17 to go v1.20.
  * `go-restfulv3`: [Go-Restful](https://github.com/emicklei/go-restful) tested v3.7.1 to 3.10.2.
  * `mux`: [Mux](https://github.com/gorilla/mux) tested v1.7.0 to v1.8.0.
  * `iris`: [Iris](https://github.com/kataras/iris) tested v12.1.0 to 12.2.5.
  * `fasthttp`: [FastHttp](https://github.com/valyala/fasthttp) tested v1.10.0 to v1.50.0.
  * `fiber`: [Fiber](https://github.com/gofiber/fiber) tested v2.49.0 to v2.50.0.
* HTTP Client
  * `http`: [Native HTTP](https://pkg.go.dev/net/http) tested go v1.17 to go v1.20.
  * `fasthttp`: [FastHttp](https://github.com/valyala/fasthttp) tested v1.10.0 to v1.50.0.
* RPC Frameworks
  * `dubbo`: [Dubbo](https://github.com/apache/dubbo-go) tested v3.0.1 to v3.0.5.
  * `kratosv2`: [Kratos](https://github.com/go-kratos/kratos) tested v2.3.1 to v2.6.2.
  * `microv4`: [Go-Micro](https://github.com/go-micro/go-micro) tested v4.6.0 to v4.10.2.
  * `grpc` : [gRPC](https://github.com/grpc/grpc-go) tested v1.55.0 to v1.57.0.
* Database Client
  * `gorm`: [GORM](https://github.com/go-gorm/gorm) tested v1.22.0 to v1.25.1.
    * [MySQL Driver](https://github.com/go-gorm/mysql)
  * `mongo`: [Mongo](https://github.com/mongodb/mongo-go-driver) tested v1.11.1 to v1.11.7.
  * `sql`: [Native SQL](https://pkg.go.dev/database/sql) tested go v1.17 to go v1.20.
    * [MySQL Driver](https://github.com/go-sql-driver/mysql) tested v1.4.0 to v1.7.1.
* Cache Client
  * `go-redisv9`: [go-redis](https://github.com/redis/go-redis) tested v9.0.3 to v9.0.5.

# Metrics Plugins
The meter plugin provides the advanced metrics collections.

* `runtimemetrics`: [Native Runtime Metrics](https://pkg.go.dev/runtime/metrics) tested go v1.17 to go v1.20.

# Logging Plugins
The logging plugin provides the advanced logging collections.

* `logrus`: [Logrus](https://github.com/sirupsen/logrus) tested v1.8.2 to v1.9.3.
* `zap`: [Zap](http://go.uber.org/zap) tested v1.17.0 to v1.24.0.