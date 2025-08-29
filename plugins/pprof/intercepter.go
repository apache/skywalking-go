package pprof

import (
	"context"
	"errors"
	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/profile"
	"runtime/pprof"
)

type SetLabelsInterceptor struct{}

func (h *SetLabelsInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	c := invocation.Args()[0].(context.Context)
	row := profile.CatchNowProfileLabel()
	if row == nil {
		return nil
	}

	now := profile.TurnToPprofLabel(row)
	if l, ok := now.(pprof.LabelSet); !ok {
		return errors.New("profile label transform error")
	} else {
		c = pprof.WithLabels(c, l)
		invocation.ChangeArg(0, c)
	}
	return nil
}

func (h *SetLabelsInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}
