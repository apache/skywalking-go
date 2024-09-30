package main

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ApiConfig struct {
	rest.RestConf

	UserRpcConf zrpc.RpcClientConf
}
type RpcConfig struct {
	zrpc.RpcServerConf
}
