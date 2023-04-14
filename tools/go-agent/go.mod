module github.com/apache/skywalking-go/tools/go-agent-enhance

go 1.18

require (
	github.com/apache/skywalking-go/plugins/core v0.0.0-20230412041451-ba963278b31e
	github.com/dave/dst v0.27.2
	github.com/sirupsen/logrus v1.9.0
)

require (
	github.com/apache/skywalking-go v0.0.0-20230412041451-ba963278b31e // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/grpc v1.54.0 // indirect
	google.golang.org/protobuf v1.29.0 // indirect
	skywalking.apache.org/repo/goapi v0.0.0-20230314034821-0c5a44bb767a // indirect
)

replace github.com/apache/skywalking-go => ../../

replace github.com/apache/skywalking-go/plugins/core => ../../plugins/core
