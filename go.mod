module github.com/apache/skywalking-go

go 1.24

toolchain go1.24.4

replace (
	skywalking.apache.org/repo/goapi => ../skywalking-goapi
//google.golang.org/genproto => google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1
//google.golang.org/genproto/googleapis/rpc => google.golang.org/genproto/googleapis/rpc v0.0.0-20230410155749-daa745c078e1
)

require (
	github.com/google/uuid v1.6.0
	github.com/pkg/errors v0.9.1
	github.com/segmentio/kafka-go v0.4.48
	google.golang.org/grpc v1.73.0
	google.golang.org/protobuf v1.36.6
	skywalking.apache.org/repo/goapi v0.0.0-20230314034821-0c5a44bb767a
)

require (
	github.com/cncf/xds/go v0.0.0-20250326154945-ae57f3c0d45f // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
//google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
)
