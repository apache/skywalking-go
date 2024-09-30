package rest

//skywalking:config gozero
var config struct {
	CollectRequestParameters bool `config:"collect_request_parameters"` // CollectRequestParameters is used to determine whether to collect request parameters.
}
