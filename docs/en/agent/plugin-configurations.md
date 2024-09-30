# Plugin Configurations

| key                                | environment key                                           | default value | description                                                                |
|------------------------------------|-----------------------------------------------------------|---------------|----------------------------------------------------------------------------|
| http.server_collect_parameters     | SW_AGENT_PLUGIN_CONFIG_HTTP_SERVER_COLLECT_PARAMETERS     | false         | Collect the parameters of the HTTP request on the server side.             |
| mongo.collect_statement            | SW_AGENT_PLUGIN_CONFIG_MONGO_COLLECT_STATEMENT            | false         | Collect the statement of the MongoDB request.                              |
| sql.collect_parameter              | SW_AGENT_PLUGIN_CONFIG_SQL_COLLECT_PARAMETER              | false         | Collect the parameter of the SQL request.                                  |
| redis.max_args_bytes               | SW_AGENT_PLUGIN_CONFIG_REDIS_MAX_ARGS_BYTES               | 1024          | Limit the bytes size of redis args request.                                |
| reporter.discard                   | SW_AGENT_REPORTER_DISCARD                                 | false         | Discard the reporter.                                                      |
| gin.collect_request_headers        | SW_AGENT_PLUGIN_CONFIG_GIN_COLLECT_REQUEST_HEADERS        |               | Collect the http header of gin request.                                    |
| gin.header_length_threshold        | SW_AGENT_PLUGIN_CONFIG_GIN_HEADER_LENGTH_THRESHOLD        | 2048          | Controlling the length limitation of all header values.                    |
| goframe.collect_request_parameters | SW_AGENT_PLUGIN_CONFIG_GOFRAME_COLLECT_REQUEST_PARAMETERS | false         | Collect the parameters of the HTTP request on the server side.             |
| goframe.collect_request_headers    | SW_AGENT_PLUGIN_CONFIG_GOFRAME_COLLECT_REQUEST_HEADERS    |               | Collect the http header of goframe request.                                |
| goframe.header_length_threshold    | SW_AGENT_PLUGIN_CONFIG_GOFRAME_HEADER_LENGTH_THRESHOLD    | 2048          | Controlling the length limitation of all header values.                    |
| gozero.collect_request_parameters  | SW_AGENT_PLUGIN_CONFIG_GOZERO_COLLECT_REQUEST_PARAMETERS  | true          | Collect the parameters of the HTTP request on the server side.             |
| gozero.collect_logx                | SW_AGENT_PLUGIN_CONFIG_GOZERO_COLLECT_LOGX                | true          | Collect the parameters of the gozero logx info.                            |