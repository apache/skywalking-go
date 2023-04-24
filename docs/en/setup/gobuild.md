# Setup in build

When you want to integrate the Agent using the original go build command, you need to follow these steps.

## Install SkyWalking Go

Use `go get` to import the `skywalking-go` program.

```shell
go get github.com/apache/skywalking-go
```

Also, import the module to your main package: 

```go
import _ "github.com/apache/skywalking-go"
```

## Build the project
When building the project, you need to download the Golang enhancement program first:

```shell
go install github.com/apache/skywalking-go/tools/go-agent
```

When using go build, add the following parameters:

```shell
-toolexec="/path/to/go-agent" -a
```

1. `-toolexec` is the path to the Golang enhancement program.
2. `-a` is the parameter for rebuilding all packages forcibly.

If you want to customize the configuration information for the current service, please add the following parameters, 
[read more please refer the settings override documentation](../advanced-features/settings-override.md)):

```shell
-toolexec="/path/to/skywalking-enhance -config /path/to/config.yaml" -a
```