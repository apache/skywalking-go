package main

import (
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"test/plugins/scenarios/gozero/user"
)

var defaultMaxRecvMsgSize = 10 * 1024 * 1024 //default 10M max receive message size

type ApiServiceContext struct {
	Config ApiConfig

	UserRpc user.User // user rpc client
}

func NewApiServiceContext(c ApiConfig) *ApiServiceContext {
	return &ApiServiceContext{
		Config: c,

		UserRpc: createUserRpc(c),
	}
}

// createUserRpc creates a rpc client for user service.
func createUserRpc(c ApiConfig) user.User {
	userClient, err := zrpc.NewClient(c.UserRpcConf, zrpc.WithDialOption(grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaultMaxRecvMsgSize))))
	if err != nil {
		logx.Error(errors.Wrap(err, "failed to create user rpc client"))
		return nil
	}
	userRpc := user.NewUser(userClient)
	return userRpc
}

type RpcServiceContext struct {
	Config RpcConfig
}

func NewRpcServiceContext(c RpcConfig) *RpcServiceContext {
	return &RpcServiceContext{
		Config: c,
	}
}
