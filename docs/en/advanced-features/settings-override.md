# Setting Override

By default, SkyWalking Go agent provides a default [agent.default.yaml](../../../tools/go-agent/config/agent.default.yaml) to define the default configuration options.

This configuration file is used **during hybrid compilation to write the configuration information of the Agent into the program**.
When the program boots, the agent would read the pre-configured content.

## Configuration Changes

The values in the config file should be updated by following the user requirements. They are applied during the hybrid compilation process.

For missing configuration items in the custom file, the Agent would use the values from the **default configuration**.

## Environment Variables

In the default configuration, you can see that most of the configurations are in the format `${xxx:config_value}`.
It means that when the program starts, the agent would first read the `xxx` from the **system environment variables** in the runtime.
If it cannot be found, the value would be used as the `config_value` as value. 

Note: **that the search for environment variables is at runtime, not compile time.**
