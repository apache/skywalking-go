# Build and use the Agent from source codes

When you want to build and use the Agent from source code, please follow these steps.

## Install SkyWalking Go

Use `go get` to import the latest version of `skywalking-go` program.

```shell
go get github.com/apache/skywalking-go@latest
```

Also, import the module to your `main` package: 

```go
import _ "github.com/apache/skywalking-go"
```

## Build the Agent

When building the project, you need to clone the project and build it.

```shell
git clone https://github.com/apache/skywalking-go.git
cd skywalking-go && make build
```

Next, you would find several versions of the Go Agent program for different systems in the **bin** directory of the current project. 
When you need to compile the program, please add the following statement with the agent program which matches your system:

```shell
-toolexec="/path/to/go-agent" -a
```

1. `-toolexec` is the path to the Golang enhancement program.
2. `-a` is the parameter for rebuilding all packages forcibly.

If you want to customize the configuration information for the current service, please add the following parameters, 
[read more please refer the settings override documentation](../advanced-features/settings-override.md)):

```shell
-toolexec="/path/to/go-agent -config /path/to/config.yaml" -a
```