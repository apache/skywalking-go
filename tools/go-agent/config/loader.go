// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package config

import (
	"embed"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed agent.default.yaml
var defaultAgentFS embed.FS

var config *Config

var EnvRegularRegex = regexp.MustCompile(`\${(?P<ENV>[_A-Z0-9]+):(?P<DEF>.*)}`)

var ConfigTypeAutomatic = "auto"

type Config struct {
	Agent    Agent    `yaml:"agent"`
	Reporter Reporter `yaml:"reporter"`
	Log      Log      `yaml:"log"`
	Plugin   Plugin   `yaml:"plugin"`
}

type Agent struct {
	ServiceName     StringValue `yaml:"service_name"`
	InstanceEnvName StringValue `yaml:"instance_env_name"`
	Sampler         StringValue `yaml:"sampler"`
	Meter           Meter       `yaml:"meter"`
	Correlation     Correlation `yaml:"correlation"`
	IgnoreSuffix    StringValue `yaml:"ignore_suffix"`
}

type Reporter struct {
	Discard StringValue  `yaml:"discard"`
	GRPC    GRPCReporter `yaml:"grpc"`
}

type Log struct {
	Type     StringValue `yaml:"type"`
	Tracing  LogTracing  `yaml:"tracing"`
	Reporter LogReporter `yaml:"reporter"`
}

type LogTracing struct {
	Enabled StringValue `yaml:"enable"`
	Key     StringValue `yaml:"key"`
}

type LogReporter struct {
	Enabled   StringValue `yaml:"enable"`
	LabelKeys StringValue `yaml:"label_keys"`
}

type Meter struct {
	CollectInterval StringValue `yaml:"collect_interval"`
}

type GRPCReporter struct {
	BackendService   StringValue     `yaml:"backend_service"`
	MaxSendQueue     StringValue     `yaml:"max_send_queue"`
	CheckInterval    StringValue     `yaml:"check_interval"`
	Authentication   StringValue     `yaml:"authentication"`
	CDSFetchInterval StringValue     `yaml:"cds_fetch_interval"`
	TLS              GRPCReporterTLS `yaml:"tls"`
}

type GRPCReporterTLS struct {
	Enable              StringValue `yaml:"enable"`
	CAPath              StringValue `yaml:"ca_path"`
	ClientKeyPath       StringValue `yaml:"client_key_path"`
	ClientCertChainPath StringValue `yaml:"client_cert_chain_path"`
	InsecureSkipVerify  StringValue `yaml:"insecure_skip_verify"`
}

type Plugin struct {
	Config   PluginConfig `yaml:"config"`
	Excluded StringValue  `yaml:"excluded"`
}

type Correlation struct {
	MaxKeyCount  StringValue `yaml:"max_key_count"`
	MaxValueSize StringValue `yaml:"max_value_size"`
}

func LoadConfig(path string) error {
	// load the default config
	defaultConfig, err := defaultAgentFS.ReadFile("agent.default.yaml")
	if err != nil {
		return err
	}
	if err1 := yaml.Unmarshal(defaultConfig, &config); err1 != nil {
		return err1
	}

	// if the path defined, then merge this two files
	if path == "" {
		return nil
	}
	definedContent, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	userConfig := &Config{}
	if err := yaml.Unmarshal(definedContent, userConfig); err != nil {
		return err
	}
	config.overwriteFrom(userConfig)

	return nil
}

func GetConfig() *Config {
	return config
}

type PluginConfig struct {
	data map[string]interface{}
}

func (c *PluginConfig) UnmarshalYAML(value *yaml.Node) error {
	result := make(map[string]interface{})
	if err := value.Decode(&result); err != nil {
		return err
	}
	c.data = result
	return nil
}

func (c *PluginConfig) ParseToStringValue(paths ...string) *StringValue {
	if len(paths) == 0 {
		return nil
	}
	res := c.data[paths[0]]
	for i := 1; i < len(paths); i++ {
		if res == nil {
			return nil
		}
		current, ok := res.(map[string]interface{})
		if !ok {
			panic("cannot identity the path: %s" + strings.Join(paths, "."))
		}
		res = current[paths[i]]
	}
	if res == nil {
		panic("the value of path is not found: " + strings.Join(paths, "."))
	}

	v := &StringValue{}
	v.UnmarshalString(fmt.Sprintf("%v", res))
	return v
}

type StringValue struct {
	EnvKey  string
	Default string
}

func (s *StringValue) UnmarshalYAML(value *yaml.Node) error {
	var val string
	if e := value.Decode(&val); e != nil {
		return e
	}
	s.UnmarshalString(val)
	return nil
}

func (s *StringValue) UnmarshalString(val string) {
	groups := EnvRegularRegex.FindStringSubmatch(val)
	if len(groups) == 0 {
		s.Default = val
		return
	}

	s.EnvKey = groups[1]
	s.Default = groups[2]
}

func (s *StringValue) ToGoStringValue() string {
	return strings.ReplaceAll(fmt.Sprintf(`func() string {
	if "%s" == "" { return "%s"}
	tmpValue := os.Getenv("%s")
	if tmpValue == "" { return "%s"}
	return tmpValue
}()`, s.EnvKey, s.Default, s.EnvKey, s.Default), "\n", ";")
}

func (s *StringValue) ToGoStringListValue() string {
	return strings.ReplaceAll(fmt.Sprintf(`func() []string {
	splitResult := func(s string) []string {
		t := strings.Split(s, ",")
		if len(t) == 1 && t[0] == "" { return nil }
		res := make([]string, 0, 0)
		for _, v := range t {
			if v != "" {
				res = append(res, v)
			}
		}
		return res
	}
	if "%s" == "" { return splitResult("%s") }
	tmpValue := os.Getenv("%s")
	if tmpValue == "" { return splitResult("%s") }
	return splitResult(tmpValue)
}()`, s.EnvKey, s.Default, s.EnvKey, s.Default), "\n", ";")
}

func (s *StringValue) ToGoStringFunction() string {
	return fmt.Sprintf("func() string { return %s }", s.ToGoStringValue())
}

func (s *StringValue) ToGoIntValue(errorMessage string) string {
	return strings.ReplaceAll(fmt.Sprintf(`func() int {
	if "%s" == "" {return %s}
	tmpValue := os.Getenv("%s")
	if tmpValue == "" {return %s}
	res, err := strconv.Atoi(tmpValue)
	if err != nil { panic(fmt.Errorf("%s", err))}
	return res
}()`,
		s.EnvKey, s.Default, s.EnvKey, s.Default, errorMessage), "\n", ";")
}

func (s *StringValue) ToGoIntFunction(errorMessage string) string {
	return fmt.Sprintf("func() int { return %s }", s.ToGoIntValue(errorMessage))
}

func (s *StringValue) ToGoFloatValue(errorMessage string) string {
	return strings.ReplaceAll(fmt.Sprintf(`func() float64 {
	if "%s" == "" {return %s}
	tmpValue := os.Getenv("%s")
	if tmpValue == "" {return %s}
	res, err := strconv.ParseFloat(tmpValue, 64)
	if err != nil { panic(fmt.Errorf("%s", err))}
	return res
}()`,
		s.EnvKey, s.Default, s.EnvKey, s.Default, errorMessage), "\n", ";")
}

func (s *StringValue) ToGoFloatFunction(errorMessage string) string {
	return fmt.Sprintf("func() float64 { return %s }", s.ToGoFloatValue(errorMessage))
}

func (s *StringValue) ToGoBoolValue() string {
	return strings.ReplaceAll(fmt.Sprintf(`func() bool {
	if "%s" == "" {return %s}
	tmpValue := os.Getenv("%s")
	if tmpValue == "" {return %s}
	return strings.EqualFold(tmpValue, "true")
}()`,
		s.EnvKey, s.Default, s.EnvKey, s.Default), "\n", ";")
}

func (s *StringValue) ToGoBoolFunction() string {
	return fmt.Sprintf("func() bool { return %s }", s.ToGoBoolValue())
}

func (s *StringValue) GetListStringResult() []string {
	val := s.Default
	if s.EnvKey != "" {
		if envValue := os.Getenv(s.EnvKey); envValue != "" {
			val = envValue
		}
	}
	if val == "" {
		return nil
	}
	return strings.Split(val, ",")
}

func (s *StringValue) overwriteFrom(other *StringValue) {
	if other.EnvKey != "" {
		s.EnvKey = other.EnvKey
	}
	if other.Default != "" {
		s.Default = other.Default
	}
}

func (c *Config) overwriteFrom(other *Config) {
	c1Value := reflect.ValueOf(c).Elem()
	c2Value := reflect.ValueOf(other).Elem()
	combineConfigFields(c1Value, c2Value)
}

func combineConfigFields(field1, field2 reflect.Value) {
	if field1.Type() != field2.Type() {
		panic("config are not the same")
	}

	if field1.Kind() == reflect.Struct {
		if s, ok := field1.Addr().Interface().(*StringValue); ok {
			s2 := field2.Addr().Interface().(*StringValue)
			s.overwriteFrom(s2)
		} else {
			for i := 0; i < field1.NumField(); i++ {
				combineConfigFields(field1.Field(i), field2.Field(i))
			}
		}
	}
}
