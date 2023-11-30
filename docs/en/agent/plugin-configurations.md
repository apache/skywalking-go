# Plugin Configurations

| key                            | environment key                                       | default value | description                                                    |
|--------------------------------|-------------------------------------------------------|---------------|----------------------------------------------------------------|
| http.server_collect_parameters | SW_AGENT_PLUGIN_CONFIG_HTTP_SERVER_COLLECT_PARAMETERS | false         | Collect the parameters of the HTTP request on the server side. |
| mongo.collect_statement        | SW_AGENT_PLUGIN_CONFIG_MONGO_COLLECT_STATEMENT        | false         | Collect the statement of the MongoDB request.                  |
| redis.max_args_bytes | SW_AGENT_PLUGIN_CONFIG_REDIS_MAX_ARGS_BYTES | 1024 | Limit the bytes size of redis args request.                  |
| sql.collect_parameter          | SW_AGENT_PLUGIN_CONFIG_SQL_COLLECT_PARAMETER          | false         | Collect the parameter of the SQL request.                      |
| reporter.discard               | SW_AGENT_REPORTER_DISCARD                             | false         | Discard the reporter.                                          |
