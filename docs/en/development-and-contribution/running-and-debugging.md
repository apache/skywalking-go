# Running and Debugging

Debugging is essential when developing plugins, as it helps you verify your plugin logic. If you want to perform debugging, follow these steps:

1. **Write test code**: Write a sample application that includes the framework content you need to test.
2. **Build the Agent**: In the project root directory, run the `make build` command to compile the Agent program into a binary file.
3. **Adjust the test program's Debug configuration**: Modify the test program's Debug configuration, which will be explained in more detail later.
4. **Launch the program and add breakpoints**: Start your sample application and add breakpoints in your plugin code where you want to pause the execution and inspect the program state.

## Write test code

Please make sure that you have imported `github.com/apache/skywalking-go` in your test code. 
You can refer to the [documentation on how to compile using go build for specific steps](../setup/gobuild.md#install-skywalking-go).

## Adjust the test program's Debug configuration

Please locate the following two paths:

1. **Go Agent**: Locate the binary file generated through `make build` in the previous step.
2. **Current project path**: Find the root directory of the current project, which will be used to search for source files in subsequent steps.

Then, please enter the following command in the **tool arguments** section of the debug configuration:
```
-toolexec '/path/to/skywalking-go-agent -debug /path/to/current-project-path' -a".
```