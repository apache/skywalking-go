# Setup in build

When you want to integrate the Agent using the original go build command, you need to follow these steps.

## Install SkyWalking Go

Use `go get` to import the `skywalking-go` program.

```shell
go get github.com/apache/skywalking-go
```

Also, import the module to your `main` package: 

```go
import _ "github.com/apache/skywalking-go"
```

## Download Agent

Download the Agent from the [official website](https://skywalking.apache.org/downloads/#GoAgent). 

**NOTICE**: Please ensure that the version of the Agent you downloaded is consistent with the version installed via `go get` in the previous section, 
to prevent errors such as missing package references during compilation.

Next, add the following parameters in `go build`:

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