package sarama

import (
	"embed"

	"github.com/hashicorp/go-version"

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
	return "sarama"
}

func (i *Instrument) BasePackage() string {
	return "github.com/Shopify/sarama"
}

func (i *Instrument) VersionChecker(pluginVersion string) bool {
	// https://github.com/Shopify/sarama/releases/tag/v1.27.0
	// KIP-42 producer and consumer interceptors were introduced since v1.27.0
	v1, _ := version.NewVersion(pluginVersion)
	v2, _ := version.NewVersion("v1.27.0")
	return v1.GreaterThanOrEqual(v2)
}

func (i *Instrument) Points() []*instrument.Point {
	return []*instrument.Point{
		{
			At: instrument.NewStaticMethodEnhance("newAsyncProducer",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "Client"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "AsyncProducer"), instrument.WithResultType(1, "error")),
			Interceptor: "AsyncProducerInterceptor",
		},
		{
			At: instrument.NewStaticMethodEnhance("newConsumer",
				instrument.WithArgsCount(1),
				instrument.WithArgType(0, "Client"),
				instrument.WithResultCount(2),
				instrument.WithResultType(0, "Consumer"), instrument.WithResultType(1, "error")),
			Interceptor: "ConsumerInterceptor",
		},
	}
}

func (i *Instrument) FS() *embed.FS {
	return &fs
}
