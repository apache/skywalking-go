# Plugin Exclusion

The plugin exclusion is used during the **compilation phase** to prevent certain plugins, as specified by their names, 
from being included in the compilation. Consequently, these excluded plugins will not generate corresponding data when the program is running, such as **Tracing**.

## Configuration

This configuration option is also located in the existing configuration files and [supports configuration based on environment variables](./settings-override.md#environment-variables). 
However, this environment variable only takes effect during the compilation phase.

The plugins name please refer to the [Support Plugins Documentation](../agent/support-plugins.md).