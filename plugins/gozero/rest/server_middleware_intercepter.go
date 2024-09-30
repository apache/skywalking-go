package rest

import (
	"fmt"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
)

type ServerMiddlewareInterceptor struct {
}

// SkyWalking middleware
var SkyWalkingMiddleware rest.Middleware = func(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		s, err := tracing.CreateEntrySpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), func(headerKey string) (string, error) {
			return request.Header.Get(headerKey), nil
		}, tracing.WithLayer(tracing.SpanLayerHTTP),
			tracing.WithTag(tracing.TagHTTPMethod, request.Method),
			tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path),
			tracing.WithComponent(5023))
		if err != nil {
			next(writer, request)
			return
		}

		defer s.End()

		// collect response data
		if config.CollectRequestParameters {
			switch request.Method {
			case http.MethodGet:
				if request.URL.RawQuery != "" {
					s.Tag(tracing.TagHTTPParams, request.URL.RawQuery)
				}
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				if err := request.ParseForm(); err == nil {
					s.Tag(tracing.TagHTTPParams, request.Form.Encode())
				}
			}
		}
		s.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", http.StatusOK))
		next(writer, request)
	}
}

// BeforeInvoke intercepts the HTTP request before invoking the handler.
func (h *ServerMiddlewareInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	server := invocation.CallerInstance().(*rest.Server)
	server.Use(SkyWalkingMiddleware)
	return nil
}

// AfterInvoke processes after the HTTP request has been handled.
func (h *ServerMiddlewareInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}
