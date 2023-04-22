package http

import (
	"embed"

	"github.com/apache/skywalking-go/plugins/core/instrument"
)

//go:embed *
var fs embed.FS

//skywalking:nocopy
type Instrument struct {
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "http"
}

func (i *Instrument) BasePackage() string {
	return "net/http"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "",
			At: instrument.NewMethodEnhance("Transport", "RoundTrip",
				instrument.WithArgsCount(1), instrument.WithArgType(0, "*Request")),
			Interceptor: "Interceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
