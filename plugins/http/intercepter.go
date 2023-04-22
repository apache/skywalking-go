package http

import (
	"fmt"
	"net/http"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type Interceptor struct {
}

func (h *Interceptor) BeforeInvoke(invocation *operator.Invocation) error {
	request := invocation.Args[0].(*http.Request)
	s, err := tracing.CreateExitSpan(fmt.Sprintf("%s:%s", request.Method, request.URL.Path), request.Host, func(headerKey, headerValue string) error {
		request.Header.Add(headerKey, headerValue)
		return nil
	}, tracing.WithLayer(tracing.SpanLayerHTTP),
		tracing.WithTag(tracing.TagHTTPMethod, request.Method),
		tracing.WithTag(tracing.TagURL, request.Host+request.URL.Path),
		tracing.WithComponent(5004))
	if err != nil {
		return err
	}
	invocation.Context = s
	return nil
}

func (h *Interceptor) AfterInvoke(invocation *operator.Invocation, result ...interface{}) error {
	if invocation.Context == nil {
		return nil
	}
	span := invocation.Context.(tracing.Span)
	res0 := result[0]
	res1 := result[1]
	if res0 != nil {
		resp := res0.(*http.Response)
		span.Tag(tracing.TagStatusCode, fmt.Sprintf("%d", resp.StatusCode))
	}
	if res1 != nil {
		err := result[1].(error)
		if err != nil {
			span.Error(err.Error())
		}
	}
	span.End()
	return nil
}
