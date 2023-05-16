# Plugin Exclusion

The plugin exclusion is used during the **compilation phase** to exclude specific plugins, through their names.
Consequently, the codes of these excluded plugins will not be weaved in, then, no relative tracing and metrics.

## Configuration


```yaml
plugin:
  # List the names of excluded plugins, multiple plugin names should be splitted by ","
  # NOTE: This parameter only takes effect during the compilation phase.
  excluded: ${SW_AGENT_PLUGIN_EXCLUDES:}
```

This configuration option is also located in the existing configuration files and [supports configuration based on environment variables](./settings-override.md#environment-variables). 
However, this environment variable only takes effect during the compilation phase.

The plugins name please refer to the [Support Plugins Documentation](../agent/support-plugins.md).
