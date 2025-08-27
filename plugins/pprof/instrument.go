package pprof

import (
	"embed"

	"github.com/apache/skywalking-go/plugins/core/instrument"
)

//go:embed *
var fs embed.FS

//skywalking:nocopy
type Instrument struct{}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) Name() string {
	return "profile" // 插件唯一名
}

func (i *Instrument) BasePackage() string {
	return "runtime/profile"
}

func (i *Instrument) VersionChecker(version string) bool {
	return true
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "",
			At: instrument.NewStaticMethodEnhance(
				"SetGoroutineLabels",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "context.Context"),
			),
			Interceptor: "SetLabelsInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
