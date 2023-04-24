# Hybrid Compilation

Hybrid compilation technology is the base of SkyWalking Go's implementation. 

It utilizes the `-toolexec` flag during Golang compilation to introduce custom programs that intercept all original files in the compilation stage. 
This allows for the modification or addition of files to be completed seamlessly.

## Toolchain in Golang

The `-toolexec` flag in Golang is a powerful feature that can be used during stages such as `build`, `test`, and others. 
When this flag is used, developers can provide a custom program or script to replace the default go tools functionality. 
This offers greater flexibility and control over the build, test, or analysis processes.

When passing this flag during a `go build`, it can intercept the execution flow of commands such as `compile`, `asm`, and `link`, 
which are required during Golang's compilation process. These commands are also referred to as the `toolchain` within Golang.

### Information about the Toolchain

The following command demonstrates the parameter information for the specified `-toolexec` program when it is invoked:

```shell
/usr/bin/skywalking-go /usr/local/opt/go/libexec/pkg/tool/darwin_amd64/compile -o /var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build452071603/b011/_pkg_.a -trimpath /var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build452071603/b011=> -p runtime -std -+ -buildid zSeDyjJh0lgXlIqBZScI/zSeDyjJh0lgXlIqBZScI -goversion go1.19.2 -symabis /var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build452071603/b011/symabis -c=4 -nolocalimports -importcfg /var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build452071603/b011/importcfg -pack -asmhdr /var/folders/wz/s5m922z15vz4fjhf5l4458xm0000gn/T/go-build452071603/b011/go_asm.h /usr/local/opt/go/libexec/src/runtime/alg.go /usr/local/opt/go/libexec/src/runtime/asan0.go ...
```

The code above demonstrates the parameters used when a custom program is executed, which mainly includes the following information:

1. **Current toolchain tool**: In this example, it is a compilation tool with the path: `/usr/local/opt/go/libexec/pkg/tool/darwin_amd64/compile`.
2. **Target file of the tool**: The final target file that the current tool needs to generate. 
3. **Package information**: The module package path information being compiled, which is the parameter value of the `-p` flag. The current package path is `runtime`.
4. **Temporary directory address**: For each compilation, the Go program would generate a corresponding temporary directory. This directory contains all the temporary files required for the compilation. 
5. **Files to be compiled**: Many `.go` file paths can be seen at the end of the command, which are the file path list of the module that needs to be compiled.

## Toolchain with SkyWalking Go Agent

SkyWalking Go Agent works by intercepting the `compile` program in the toolchain and making changes to the program based on the information above. The main parts include:

1. **AST**: Using `AST` to parse or modify files which ready for compiled.
2. **File copying/generation**: Copy or generate files to the temporary directory required for the compilation, and add file path addresses when the compilation command is executed.
3. **Proxy command execution**: After completing the modification of the specified package, the command execution in the toolchain will be proxied.

### Hybrid Compilation

After enhancing the program with SkyWalking Go Agent, the following parts of the program will be enhanced:

1. **SkyWalking Go**: The agent core part of the code would be dynamically copied to the agent path for plugin use.
2. **Plugins**: Enhance the specified framework code according to the enhancement rules of the plugins.
3. **Runtime**: Enhance the `runtime` package in Go, including extensions for goroutines and other content.
4. **Main**: Enhance the `main` package during system startup, for stating the system with Agent.

