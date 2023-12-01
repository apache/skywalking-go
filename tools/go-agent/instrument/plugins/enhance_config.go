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

package plugins

import (
	"fmt"
	"go/token"
	"reflect"
	"strings"
	"unicode"

	"github.com/apache/skywalking-go/plugins/core/instrument"
	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
)

var (
	configFieldKeyTag = "config"
	toolsImports      = "github.com/apache/skywalking-go/plugins/core/tools"
)

type ConfigEnhance struct {
	VarSpec               *dst.ValueSpec
	ConfigPrefix          string
	VarName               string
	Fields                []*ConfigField
	GenerateConfigFunName string
}

func NewConfigEnhance(varSpec *dst.ValueSpec, node dst.Node, inst instrument.Instrument) (*ConfigEnhance, error) {
	configDirective := tools.FindDirective(node, consts.DirectiveConfig)
	if configDirective == "" {
		return nil, fmt.Errorf("cannot find the config directive")
	}
	enhance := &ConfigEnhance{VarSpec: varSpec}
	info := strings.SplitN(configDirective, " ", 2)
	if len(info) == 2 {
		enhance.ConfigPrefix = info[1]
	} else {
		// default using the plugin name as the prefix
		enhance.ConfigPrefix = inst.Name()
	}
	enhance.VarName = varSpec.Names[0].Name

	varType, ok := varSpec.Type.(*dst.StructType)
	if !ok {
		return nil, fmt.Errorf("the config type of %s under %s plugin must be a structure", varSpec.Names[0].Name, inst.Name())
	}
	fs, err := NewConfigFields(varType)
	if err != nil {
		return nil, fmt.Errorf("analyzing the config %s under %s plugin failed: %v", varSpec.Names[0].Name, inst.Name(), err)
	}
	enhance.Fields = fs

	enhance.GenerateConfigFunName = fmt.Sprintf("initConfig%s", enhance.VarName)
	return enhance, nil
}

func (e *ConfigEnhance) PackageName() string {
	return ""
}

func (e *ConfigEnhance) BuildImports(decl *dst.GenDecl) {
	for _, spec := range decl.Specs {
		imp, ok := spec.(*dst.ImportSpec)
		if !ok {
			continue
		}
		if imp.Path.Value == fmt.Sprintf("%q", toolsImports) {
			return
		}
	}
	decl.Specs = append(decl.Specs, &dst.ImportSpec{
		Path: &dst.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("%q", toolsImports),
		},
	})
}

func (e *ConfigEnhance) BuildForDelegator() []dst.Decl {
	fun := &dst.FuncDecl{
		Name: dst.NewIdent(e.GenerateConfigFunName),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	for _, field := range e.Fields {
		fun.Body.List = append(fun.Body.List, field.GenerateAssignFieldValue(e.VarName, []string{}, []string{e.ConfigPrefix})...)
	}
	return []dst.Decl{fun}
}

func (e *ConfigEnhance) ReplaceFileContent(path, content string) string {
	return content
}

func (e *ConfigEnhance) InitFunctions() []*EnhanceInitFunction {
	return []*EnhanceInitFunction{NewEnhanceInitFunction(e.GenerateConfigFunName, true)}
}

type ConfigField struct {
	Name        string
	Type        string
	Key         string
	ChildFields []*ConfigField
}

func NewConfigFields(structType *dst.StructType) ([]*ConfigField, error) {
	if structType.Fields == nil || len(structType.Fields.List) == 0 {
		return nil, fmt.Errorf("the config structure must have at least one field")
	}
	fields := make([]*ConfigField, 0, len(structType.Fields.List))
	for _, field := range structType.Fields.List {
		configField, err := NewConfigField(field)
		if err != nil {
			return nil, err
		}
		fields = append(fields, configField)
	}

	return fields, nil
}

func NewConfigField(f *dst.Field) (*ConfigField, error) {
	if len(f.Names) == 0 {
		return nil, fmt.Errorf("the config structure must have named field")
	}
	conf := &ConfigField{
		Name: f.Names[0].Name,
	}
	switch t := f.Type.(type) {
	case *dst.Ident:
		conf.Type = t.Name
	case *dst.StructType:
		fs, err := NewConfigFields(t)
		if err != nil {
			return nil, err
		}
		conf.ChildFields = fs
	default:
		return nil, fmt.Errorf("the config structure field %s type %T is not supported", conf.Name, t)
	}
	conf.initFlags(f.Tag)
	return conf, nil
}

func (f *ConfigField) initFlags(flagLit *dst.BasicLit) {
	if flagLit == nil {
		f.Key = f.generateDefaultKey(f.Name)
		return
	}

	tag := reflect.StructTag(flagLit.Value)
	value, ok := tag.Lookup(configFieldKeyTag)
	if ok {
		f.Key = value
		return
	}

	f.Key = f.generateDefaultKey(f.Name)
}

func (f *ConfigField) generateDefaultKey(keyName string) string {
	var result []rune
	for i, r := range keyName {
		if unicode.IsUpper(r) {
			if i != 0 {
				result = append(result, '_')
			}
			result = append(result, []rune(strings.ToLower(string(r)))...)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func (f *ConfigField) GenerateAssignFieldValue(varName string, field, path []string) []dst.Stmt {
	field = append(field, f.Name)
	path = append(path, f.Key)
	if len(f.ChildFields) > 0 {
		result := make([]dst.Stmt, 0)
		for _, child := range f.ChildFields {
			result = append(result, child.GenerateAssignFieldValue(varName, field, path)...)
		}
		return result
	}
	fieldKeyPathStr := strings.Join(path, ".")
	fieldPathStr := strings.Join(field, ".")
	pluginConfig := config.GetConfig().Plugin.Config.ParseToStringValue(path...)
	if pluginConfig == nil {
		panic(fmt.Errorf("cannot find the config %s", fieldKeyPathStr))
	}
	resultType := f.Type
	getFromEnvStr := ""
	if pluginConfig.EnvKey != "" {
		getFromEnvStr = fmt.Sprintf("if v := tools.GetEnvValue(%q); v != \"\" { result = v };", pluginConfig.EnvKey)
	}
	parseResStr := ""
	parseErrorMessage := fmt.Sprintf(`"cannot parse the config %s: " + err.Error()`, fieldKeyPathStr)
	switch f.Type {
	case "string":
		parseResStr = "return result"
	case "bool":
		parseResStr = "return tools.ParseBool(result)"
	case "int":
		parseResStr = "if v, err := tools.Atoi(result); err != nil { panic(" + parseErrorMessage + ") } else { return v }"
	case "int16":
		parseResStr = "if v, err := tools.ParseInt(result, 10, 16); err != nil { panic(" + parseErrorMessage + ") } else { return v }"
	case "int32":
		parseResStr = "if v, err := tools.ParseInt(result, 10, 32); err != nil { panic(" + parseErrorMessage + ") } else { return v }"
	case "int64":
		parseResStr = "if v, err := tools.ParseInt(result, 10, 64); err != nil { panic(" + parseErrorMessage + ") } else { return v }"
	case "float32":
		parseResStr = "if v, err := tools.ParseFloat(result, 32); err != nil { panic(" + parseErrorMessage + ") } else { return v }"
	case "float64":
		parseResStr = "if v, err := tools.ParseFloat(result, 64); err != nil { panic(" + parseErrorMessage + ") } else { return v }"
	case "float":
		parseResStr = "if v, err := tools.ParseFloat(result, 64); err != nil { panic(" + parseErrorMessage + ") } else { return v }"
	default:
		panic("unsupported config type " + f.Type)
	}
	stmtStr := fmt.Sprintf("%s.%s = func () %s { result := %q; %s%s }()",
		varName, fieldPathStr, resultType, pluginConfig.Default, getFromEnvStr, parseResStr)
	return tools.GoStringToStats(stmtStr)
}
