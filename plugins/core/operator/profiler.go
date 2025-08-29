package operator

type ProfileOperator interface {
	GetPprofLabelSet() interface{}
	TurnToPprofLabel(l interface{}) interface{}
}
