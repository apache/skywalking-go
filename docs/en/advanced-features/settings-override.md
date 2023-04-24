# Setting Override

By default, SkyWalking Go provides a default [agent.default.yaml](../../../tools/go-agent/config/agent.default.yaml) to define the default configuration options.

This configuration file is used **during hybrid compilation to write the configuration information of the Agent into the program**. 
When the program starts, agent would read the pre-configured content.

## Configuration Changes

If you want to modify the default configuration, you can copy the default file and specify it as the file when building.

For missing configuration items in the custom file, the Agent would use the values from the **default configuration**.

## Environment Variables

In the default configuration, you can see that most of the configurations are in the format `${xxx:config_value}`. 
It means that when the program starts, agent would first read the `xxx` from the **system environment variables**. 
If it cannot be found, the value would be used as the `config_value` as value. 

Note: **that the search for environment variables is at runtime, not compile time.**
