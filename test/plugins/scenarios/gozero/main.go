package main

import (
	"fmt"
	_ "github.com/apache/skywalking-go"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"sync"
	"test/plugins/scenarios/gozero/pb/userpb"
	"time"
)

func main() {
	apiConfig := ApiConfig{}
	rpcConfig := RpcConfig{}
	// initConfig
	initConfig(&apiConfig, &rpcConfig)

	var wg sync.WaitGroup
	wg.Add(2)
	// ------------rpcServer grpc server-----------
	go func() {
		defer wg.Done()
		rpcCtx := NewRpcServiceContext(rpcConfig)
		rpcServer := zrpc.MustNewServer(rpcConfig.RpcServerConf, func(grpcServer *grpc.Server) {
			userpb.RegisterUserServer(grpcServer, NewUserRpcServer(rpcCtx))

			if rpcConfig.Mode == service.DevMode || rpcConfig.Mode == service.TestMode {
				reflection.Register(grpcServer)
			}
		})
		defer rpcServer.Stop()
		fmt.Printf("Starting rpc server at %s...\n", rpcConfig.ListenOn)
		rpcServer.Start()
	}()

	// Ensure rpcServer starts before restServer
	time.Sleep(1 * time.Second)

	// ------------restServer http server-----------
	go func() {
		defer wg.Done()
		restServer := rest.MustNewServer(apiConfig.RestConf)
		defer restServer.Stop()

		apiCtx := NewApiServiceContext(apiConfig)
		RegisterApiHandlers(restServer, apiCtx)
		fmt.Printf("Starting api server at %s:%d...\n", apiConfig.Host, apiConfig.Port)
		restServer.Start()
	}()

	wg.Wait()
}

// initConfig initializes the configuration.
func initConfig(apiConf *ApiConfig, rpcConf *RpcConfig) {
	// restConf
	apiConf.RestConf = rest.RestConf{
		Host: "0.0.0.0",
		Port: 8888,
		ServiceConf: service.ServiceConf{
			Mode: service.DevMode,
		},
		TraceIgnorePaths: []string{"/health"},
	}
	// userRpcConf
	apiConf.UserRpcConf = zrpc.RpcClientConf{
		Endpoints: []string{"127.0.0.1:8889"},
		Timeout:   30000,
	}
	// rpcServerConf
	rpcConf.RpcServerConf = zrpc.RpcServerConf{
		ListenOn: "0.0.0.0:8889",
		ServiceConf: service.ServiceConf{
			Mode: service.DevMode,
		},
	}
}
