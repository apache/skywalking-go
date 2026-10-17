module github.com/apache/skywalking-go/plugins/core

go 1.25.0

require (
	github.com/apache/skywalking-go v0.7.0
	github.com/dave/dst v0.27.2
	github.com/google/uuid v1.6.0
	github.com/pkg/errors v0.9.1
	github.com/segmentio/kafka-go v0.4.47
	github.com/stretchr/testify v1.8.2
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Protocols live in the parent module. Keep the local tree when this module is
// tidied outside the workspace.
replace github.com/apache/skywalking-go => ../..
