package pprof

import (
	"fmt"
	"github.com/apache/skywalking-go/plugins/core/operator"
)

type SetLabelsInterceptor struct{}

func (h *SetLabelsInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	//c := invocation.Args()[0].(context.Context)
	////var c = context.Background()
	//nl := GetPprofLabelSet()
	//if nl == nil {
	//	return nil
	//}
	//r := TurnToPprofLabel(nl)
	//pprof.WithLabels(c, r)
	fmt.Println("Label Interceptor BeforeInvoke")
	return nil
}

func (h *SetLabelsInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	fmt.Println("Label Interceptor AfterInvoke")
	return nil
}
