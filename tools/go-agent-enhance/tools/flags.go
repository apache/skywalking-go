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

package tools

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

const flagTagKey = "skyflag"

func ParseFlags(result interface{}, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no args")
	}
	flags := parseFlagsFromStruct(result)
	if len(flags) == 0 {
		return nil
	}

	i := 0
	for i < len(args)-1 {
		i += parseFlag(flags, args[i], args[i+1])
	}

	if i < len(args) {
		parseFlag(flags, args[i], "")
	}
	return nil
}

func ParseProxyCommandName(args []string) string {
	if len(args) == 0 {
		return ""
	}

	cmd := filepath.Base(args[0])
	if ext := filepath.Ext(cmd); ext != "" {
		cmd = strings.TrimSuffix(cmd, ext)
	}
	return cmd
}

func parseFlag(flags map[string]reflect.Value, curArg, nextArg string) int {
	if curArg[0] != '-' {
		return 1
	}

	kv := strings.SplitN(curArg, "=", 2)
	option := kv[0]
	if v, exist := flags[option]; !exist {
		if len(kv) == 2 {
			return 1
		} else if nextArg == "" || (len(nextArg) > 1 && nextArg[0] != '-') {
			return 2
		}
		return 1
	} else if len(kv) == 2 {
		v.SetString(kv[1])
		return 1
	} else {
		switch v.Kind() {
		case reflect.String:
			v.SetString(nextArg)
			return 2
		case reflect.Bool:
			v.SetBool(true)
			return 1
		}
		return 1
	}
}

func parseFlagsFromStruct(result interface{}) map[string]reflect.Value {
	e := reflect.ValueOf(result).Elem()
	typ := e.Type()
	flagSetValueMap := make(map[string]reflect.Value, e.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if tag, ok := field.Tag.Lookup(flagTagKey); ok {
			flagSetValueMap[tag] = e.Field(i)
		}
	}
	return flagSetValueMap
}
