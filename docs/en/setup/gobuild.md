# Setup in build

When you want to integrate the Agent using the original go build command, you need to follow these steps.

## 1. Download Agent

Download the Agent from the [official website](https://skywalking.apache.org/downloads/#GoAgent).

## 2. Install SkyWalking Go

SkyWalking Go offers two ways for integration into your project.

### 2.1 Agent Injector

Agent injector is recommended when you only want to include SkyWalking Go agent in the compiling pipeline or shell.

Please execute the following command, which would automatically import SkyWalking Go into your project.

```shell
/path/to/agent -inject /path/to/your/project [-all]
```

* `/path/to/agent` is the path to the agent which your downloaded.
* `/path/to/your/project` is the home path to your project, support absolute and related with current directory path.
* `-all` is the parameter for injecting all submodules in your project.

### 2.2 Code Dependency

Use `go get` to import the `skywalking-go` program.

```shell
go get github.com/apache/skywalking-go
```

Also, import the module to your `main` package: 

```go
import _ "github.com/apache/skywalking-go"
```

**NOTICE**: Please ensure that the version of the Agent you downloaded is consistent with the version installed via `go get` in the previous section,
to prevent errors such as missing package references during compilation.

## 3. Build with SkyWalking Go Agent

Add the following parameters in `go build`:

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

# Binary Output
The binary would be weaved and instrumented by SkyWalking Go.
