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

const flagTagKey = "swflag"

func ParseFlags(result interface{}, args []string) (noOpIndex int, err error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("no args")
	}
	flags := parseFlagsFromStruct(result)
	if len(flags) == 0 {
		return 0, nil
	}

	i := 0
	firstNonOptionIndex := -1
	for i < len(args)-1 {
		shift, isOption := parseFlag(flags, args[i], args[i+1])
		if !isOption && firstNonOptionIndex == -1 {
			firstNonOptionIndex = i
		}
		i += shift
	}

	if i < len(args) {
		parseFlag(flags, args[i], "")
	}

	// process the all args flag
	if v, exist := flags["all-args"]; exist {
		v.Set(reflect.ValueOf(args))
	}
	return firstNonOptionIndex, nil
}

func ParseProxyCommandName(args []string, firstNonOptionIndex int) string {
	if len(args) == 0 {
		return ""
	}

	cmd := filepath.Base(args[firstNonOptionIndex])
	if ext := filepath.Ext(cmd); ext != "" {
		cmd = strings.TrimSuffix(cmd, ext)
	}
	return cmd
}

func parseFlag(flags map[string]reflect.Value, curArg, nextArg string) (shift int, isOption bool) {
	if curArg[0] != '-' {
		return 1, false
	}

	kv := strings.SplitN(curArg, "=", 2)
	option := kv[0]
	if v, exist := flags[option]; !exist {
		if len(kv) == 2 {
			return 1, true
		} else if nextArg == "" || (len(nextArg) > 1 && nextArg[0] != '-') {
			return 2, true
		}
		return 1, true
	} else if len(kv) == 2 {
		v.SetString(kv[1])
		return 1, true
	} else {
		switch v.Kind() {
		case reflect.String:
			v.SetString(nextArg)
			return 2, true
		case reflect.Bool:
			v.SetBool(true)
			return 1, true
		}
		return 1, true
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
