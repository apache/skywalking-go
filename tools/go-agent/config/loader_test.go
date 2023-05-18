package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	err := LoadConfig("./testdata/agent.yaml")
	assert.NoError(t, err)
	conf := GetConfig()
	assert.NotNil(t, conf)
	expected := StringValue{EnvKey: "SW_AGENT_NAME", Default: "test-service"}
	assert.Equal(t, expected, conf.Agent.ServiceName)
	expected = StringValue{EnvKey: "SW_AGENT_SAMPLE", Default: "0.1"}
	assert.Equal(t, expected, conf.Agent.Sampler)
}
