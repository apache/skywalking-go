package main

import (
	"context"
	"errors"
	"github.com/zeromicro/go-zero/core/logx"
	"test/plugins/scenarios/gozero/pb/userpb"
	"time"
)

// user api logic
type UserApiLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *ApiServiceContext
}

// NewUserApiLogic create user api logic
func NewUserApiLogic(ctx context.Context, svcCtx *ApiServiceContext) *UserApiLogic {
	return &UserApiLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserApiLogic) UserFind(req *UserFindReq) (resp *UserInfo, err error) {
	logx.Infof("user api find: %+v", req)
	logx.Error("test error log")
	if l.svcCtx.UserRpc == nil {
		return nil, errors.New("user rpc client not initialize")
	}

	findResp, err := l.svcCtx.UserRpc.UserFind(l.ctx, &userpb.UserFindReq{Id: 888})
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		Id:        findResp.Id,
		Uuid:      findResp.Uuid,
		Name:      findResp.Name,
		Mobile:    findResp.Mobile,
		Email:     findResp.Email,
		Status:    1,
		CreatedAt: findResp.CreatedAt,
		UpdatedAt: findResp.UpdatedAt,
	}, nil
}

func (l *UserApiLogic) UserSave(req *UserSaveReq) (resp *UserInfo, err error) {
	logx.Infof("user api save: %+v", req)

	return &UserInfo{
		Id:        req.Id,
		Uuid:      req.Uuid,
		Name:      req.Name,
		Mobile:    req.Mobile,
		Email:     req.Email,
		Status:    req.Status,
		CreatedAt: time.Now().Format(time.DateTime),
		UpdatedAt: time.Now().Format(time.DateTime),
	}, nil
}

// user rpc logic
type UserRpcLogic struct {
	ctx    context.Context
	svcCtx *RpcServiceContext
	logx.Logger
}

func NewUserRpcLogic(ctx context.Context, svcCtx *RpcServiceContext) *UserRpcLogic {
	return &UserRpcLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// user find rpc logic
func (l *UserRpcLogic) UserFind(in *userpb.UserFindReq) (*userpb.UserInfo, error) {
	logx.Infof("user rpc find: %+v", in)

	return &userpb.UserInfo{
		Id:        in.Id,
		Uuid:      "abd",
		Name:      "jack",
		Mobile:    "12345678901",
		Email:     "test@163.om",
		CreatedAt: time.Now().Format(time.DateTime),
		UpdatedAt: time.Now().Format(time.DateTime),
	}, nil
}

// user save rpc logic
func (l *UserRpcLogic) UserSave(in *userpb.UserSaveReq) (*userpb.UserInfo, error) {
	// todo: add your logic here and delete this line

	return &userpb.UserInfo{}, nil
}
