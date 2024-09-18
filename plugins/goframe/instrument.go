package goframe

import (
	"embed"
	"github.com/apache/skywalking-go/plugins/core/instrument"
	"strings"
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
	return "goframe"
}

func (i *Instrument) BasePackage() string {
	return "github.com/gogf/gf/v2"
}

func (i *Instrument) VersionChecker(version string) bool {
	return strings.HasPrefix(version, "v2.")
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			PackagePath: "net/ghttp",
			At: instrument.NewMethodEnhance("*Server", "ServeHTTP",
				instrument.WithArgsCount(2), instrument.WithArgType(0, "http.ResponseWriter"),
				instrument.WithArgType(1, "*http.Request")),
			Interceptor: "GoFrameServerInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
