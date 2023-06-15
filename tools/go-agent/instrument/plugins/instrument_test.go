package plugins

import (
	"embed"
	"testing"

	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
)

func TestInstrument_tryToFindThePluginVersion(t *testing.T) {
	tests := []struct {
		name string
		opts *api.CompileOptions
		ins  instrument.Instrument
		want string
	}{
		{
			"normal plugin path",
			&api.CompileOptions{
				AllArgs: []string{
					"github.com/gin-gonic/gin@1.1.1/gin.go",
				},
			},
			NewTestInstrument("github.com/gin-gonic/gin"),
			"1.1.1",
		},
		{
			"plugin with upper-case path",
			&api.CompileOptions{
				AllArgs: []string{
					"github.com/!shopify/sarama@1.34.1/acl.go",
				},
			},
			NewTestInstrument("github.com/Shopify/sarama"),
			"1.34.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &Instrument{}
			got, _ := i.tryToFindThePluginVersion(tt.opts, tt.ins)
			if got != tt.want {
				t.Errorf("tryToFindThePluginVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}

type TestInstrument struct {
	basePackage string
}

func NewTestInstrument(basePackage string) *TestInstrument {
	return &TestInstrument{basePackage: basePackage}
}

func (i *TestInstrument) Name() string {
	return ""
}

func (i *TestInstrument) BasePackage() string {
	return i.basePackage
}

func (i *TestInstrument) VersionChecker(version string) bool {
	return true
}

func (i *TestInstrument) Points() []*instrument.Point {
	return []*instrument.Point{}
}

func (i *TestInstrument) FS() *embed.FS {
	return nil
}
