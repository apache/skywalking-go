package main

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
)

func RegisterApiHandlers(server *rest.Server, serverCtx *ApiServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method: http.MethodGet,
				Path:   "/health",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					httpx.OkJson(w, "ok")
				},
			},
		},
	)
	server.AddRoutes(
		[]rest.Route{
			{
				// user find handler
				Method:  http.MethodGet,
				Path:    "/info",
				Handler: UserFindHandler(serverCtx),
			},
			{
				// user save handler
				Method:  http.MethodPost,
				Path:    "/save",
				Handler: UserSaveHandler(serverCtx),
			},
		},
		rest.WithPrefix("/user"),
	)
}

// user find
func UserFindHandler(svcCtx *ApiServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req UserFindReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := NewUserApiLogic(r.Context(), svcCtx)
		resp, err := l.UserFind(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

// user save
func UserSaveHandler(svcCtx *ApiServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req UserSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := NewUserApiLogic(r.Context(), svcCtx)
		resp, err := l.UserSave(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

type UserFindReq struct {
	Id   int64  `form:"id,optional"`
	Uuid string `form:"uuid,optional"`
}

type UserInfo struct {
	Id        int64  `json:"id"`
	Uuid      string `json:"uuid"`
	Name      string `json:"name"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	Status    int64  `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type UserSaveReq struct {
	Id       int64  `json:"id,optional" form:"id,optional"`
	Uuid     string `json:"uuid,optional" form:"uuid,optional"`
	Name     string `json:"name,optional" form:"name,optional"`
	Mobile   string `json:"mobile,optional" form:"mobile,optional"`
	Email    string `json:"email,optional" form:"email,optional"`
	Status   int64  `json:"status,optional" form:"status,optional"`
	Password string `json:"password,optional" form:"password,optional"`
}
