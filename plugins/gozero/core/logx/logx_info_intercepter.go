package logx

import (
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type LoggerInfoInterceptor struct {
}

// BeforeInvoke implements the instrument.Interceptor interface.
func (h *LoggerInfoInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	if config.CollectLogx {
		span, err := createLogxSpan(invocation, "info")
		if err != nil {
			return err
		}
		invocation.SetContext(span)
	}
	return nil
}

// AfterInvoke implements the instrument.Interceptor interface.
func (h *LoggerInfoInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	if invocation.GetContext() != nil {
		span := invocation.GetContext().(tracing.Span)
		span.End()
	}
	return nil
}
