# Write Plugin Test

Writing plugin test cases can greatly help you determine if your plugin is running well across multiple versions. 
If you haven't started developing your plugin yet, please read this [Plugin Development Guide](./development-guide.md) first.

Developing a plugin involves the following steps:

1. **Create a new module**: Please create a new module in the [specified directory](../../../test/plugins/scenarios), and it is recommended to name the module the same as the plugin for easy reference.
2. **Write the configuration file**: This file serves as the declaration file for the plugin, and test cases would be run based on this file.
3. **Write the test code**: Simulate the actual service operation, including the plugin you want to test.
4. **Test execution**: Check if the plugin is running properly.

## Write Configuration File

The configuration file is used to define the basic information of the test plugin. 
You can use [the gin plugin configuration file](../../../test/plugins/scenarios/gin/plugin.yml) as an example to write your own. 
It includes the following information:

1. **entry-service**: The test HTTP service entry URL. When this address is accessed, the plugin code should be triggered.
2. **health-checker**: Executed before the **entry-service** is accessed to ensure that the service starts without any issues. Status code of `200` is considered a successful service start.
3. **start-script**: The script execution file path. Please compile and start the service in this file.
4. **framework**: The access address of the current framework to be tested. During testing, this address would be used to switch between different framework versions.
5. **export-port**: The port number for the external service entry.
6. **support-version**: The version information supported by the current plugin.
   1. **go**: The supported Golang language version for the current plugin.
   2. **framework**: A list of plugin version information. It would be used to switch between multiple framework versions.

### URL Access

When the service address is accessed, please use `${HTTP_HOST}` and `${HTTP_PORT}` to represent the domain name and port number to be accessed. 
The port number corresponds to the **export-port** field.

### Start Script

The startup script is used to compile and execute the program.

When starting, please add the `${GO_BUILD_OPTS}` parameter, which specifies the Go Agent program information for hybrid compilation.

When starting, just let the program keep running.

### Version Matrix

Multi-version support is a crucial step in plugin testing. It can test whether the plugin runs stably across multiple framework versions and go versions.

Plugin testing would use the `go get` command to modify the plugin version. Please make sure you have filled in the correct **framework** and **support-version.framework**.
The format is: `${framework}@${support-version.framework}`

During plugin execution, the specified official Golang image would be used, allowing the plugin to run in the designated Golang version.

### Excepted File

For each plugin, you need to define the **config/expected.yml** file, which is used to define the observable data generated after the plugin runs. 
After the plugin runs, this file would be used to validate the data. 

Please refer to the [documentation](https://skywalking.apache.org/docs/skywalking-java/next/en/setup/service-agent/java-agent/plugin-test/#expecteddatayaml) to write this file.

## Write Test Code

In the test code, please start an HTTP service and expose the following two interfaces:

1. **Check service**: Used to ensure that the service is running properly. This corresponds to the **health-checker** address in configuration.
2. **Entry service**: Write the complete framework business logic at this address. Validate all the features provided by the plugin as much as possible.
This corresponds to the **entry-service** address in configuration.

The test code, like a regular program, needs to import the `github.com/apache/skywalking-go` package.

## Test Execution

Once you have completed the plugin configuration and test code writing, you can proceed to test the framework. Please follow these steps:

1. **Build tools**: Execute the `make build` command in the [test/plugins](../../../test/plugins) directory. It would generate some tools needed for testing in the `dist` folder of this directory.
2. **Run the plugin locally**: Start the plugin test program and iterate through all framework versions for testing on your local environment.
3. **Add to GitHub Action**: Fill in the name of the test plugin [in this file](../../../.github/workflows/plugin-tests.yaml), and the plugin test would be executed and validated each time a pull request is submitted.

### Run the Plugin Test Locally

Please execute the **run.sh** script in the [test/plugins directory](../../../test/plugins) and pass in the name of the plugin you wrote (the folder name). 
At this point, the script would read the configuration file of the plugin test and create a workspace directory in this location for temporarily storing files generated by each plugin. 
Finally, it would start the test code and validate the data sequentially according to the supported version information.

The script supports the following two parameters:

1. **--clean**: Clean up the files and containers generated by the current running environment.
2. **--debug**: Enable debug mode for plugin testing. In this mode, the content generated by each framework in the workspace would not be cleared, and the temporary files generated during hybrid compilation would be saved.