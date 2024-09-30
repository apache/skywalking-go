package logx

import (
	"fmt"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type LoggerErrorInterceptor struct {
}

var createLogxSpan = func(invocation operator.Invocation, operation string) (tracing.Span, error) {
	argLen := len(invocation.Args())
	if argLen == 0 {
		return nil, nil
	}
	var logxData string
	if argLen == 1 {
		logxData = fmt.Sprint(invocation.Args()[0].([]any)...)
	} else {
		logxData = fmt.Sprintf(invocation.Args()[0].(string), invocation.Args()[1].([]any)...)
	}
	return tracing.CreateLocalSpan(fmt.Sprintf("logx/%s", operation), tracing.WithTag("content", logxData), tracing.WithComponent(5023))
}

// BeforeInvoke implements the instrument.Interceptor interface.
func (h *LoggerErrorInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	if config.CollectLogx {
		span, err := createLogxSpan(invocation, "error")
		if err != nil {
			return err
		}
		invocation.SetContext(span)
	}
	return nil
}

// AfterInvoke implements the instrument.Interceptor interface.
func (h *LoggerErrorInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() != nil {
		span := invocation.GetContext().(tracing.Span)
		span.End()
	}
	return nil
}
