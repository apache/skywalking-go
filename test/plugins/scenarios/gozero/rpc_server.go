package main

import (
	"context"
	"test/plugins/scenarios/gozero/pb/userpb"
)

type UserRpcServer struct {
	svcCtx *RpcServiceContext
	userpb.UnimplementedUserServer
}

// NewUserRpcServer new a rpc server
func NewUserRpcServer(svcCtx *RpcServiceContext) *UserRpcServer {
	return &UserRpcServer{
		svcCtx: svcCtx,
	}
}

// user find
func (s *UserRpcServer) UserFind(ctx context.Context, in *userpb.UserFindReq) (*userpb.UserInfo, error) {
	l := NewUserRpcLogic(ctx, s.svcCtx)
	return l.UserFind(in)
}

// user save
func (s *UserRpcServer) UserSave(ctx context.Context, in *userpb.UserSaveReq) (*userpb.UserInfo, error) {
	l := NewUserRpcLogic(ctx, s.svcCtx)
	return l.UserSave(in)
}
