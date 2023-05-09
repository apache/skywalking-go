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
}

type Agent struct {
	ServiceName     StringValue `yaml:"service_name"`
	InstanceEnvName StringValue `yaml:"instance_env_name"`
	Sampler         StringValue `yaml:"sampler"`
}

type Reporter struct {
	GRPC GRPCReporter `yaml:"grpc"`
}

type Log struct {
	Type    StringValue `yaml:"type"`
	Tracing LogTracing  `yaml:"tracing"`
}

type LogTracing struct {
	Enabled StringValue `yaml:"enable"`
	Key     StringValue `yaml:"key"`
}

type GRPCReporter struct {
	BackendService StringValue `yaml:"backend_service"`
	MaxSendQueue   StringValue `yaml:"max_send_queue"`
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

type StringValue struct {
	EnvKey  string
	Default string
}

func (s *StringValue) UnmarshalYAML(value *yaml.Node) error {
	var val string
	if e := value.Decode(&val); e != nil {
		return e
	}

	groups := EnvRegularRegex.FindStringSubmatch(val)
	if len(groups) == 0 {
		s.Default = val
		return nil
	}

	s.EnvKey = groups[1]
	s.Default = groups[2]
	return nil
}

func (s *StringValue) ToGoStringValue() string {
	return strings.ReplaceAll(fmt.Sprintf(`func() string {
	if "%s" == "" { return "%s"}
	tmpValue := os.Getenv("%s")
	if tmpValue == "" { return "%s"}
	return tmpValue
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

func (s *StringValue) overwriteFrom(other StringValue) {
	if other.EnvKey != "" {
		s.EnvKey = other.EnvKey
	}
	if other.Default != "" {
		s.Default = other.Default
	}
}

func (c *Config) overwriteFrom(other *Config) {
	c1Value := reflect.ValueOf(&c).Elem()
	c2Value := reflect.ValueOf(&other).Elem()
	combineConfigFields(c1Value, c2Value)
}

func combineConfigFields(field1, field2 reflect.Value) {
	if field1.Type() != field2.Type() {
		panic("config are not the same")
	}

	if field1.Kind() == reflect.Struct {
		if s, ok := field1.Addr().Interface().(StringValue); ok {
			s2 := field2.Addr().Interface().(StringValue)
			s.overwriteFrom(s2)
		} else {
			for i := 0; i < field1.NumField(); i++ {
				combineConfigFields(field1.Field(i), field2.Field(i))
			}
		}
	}
}
