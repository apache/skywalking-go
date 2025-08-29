package profile

import "github.com/apache/skywalking-go/plugins/core/operator"

func CatchNowProfileLabel() interface{} {
	op := operator.GetOperator()
	if op == nil {
		return nil
	}
	profiler, ok := op.Profiler().(operator.ProfileOperator)
	if !ok {
		return nil
	}
	re := profiler.GetPprofLabelSet()
	return re
}

func TurnToPprofLabel(t interface{}) interface{} {
	op := operator.GetOperator()
	if op == nil {
		return nil
	}
	profiler, ok := op.Profiler().(operator.ProfileOperator)
	if !ok {
		return nil
	}
	re := profiler.TurnToPprofLabel(t)
	return re
}
