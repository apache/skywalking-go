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

package core

import (
	"reflect"
	"unsafe"
)

type TracerTools struct {
}

func NewTracerTools() *TracerTools {
	return &TracerTools{}
}

type ReflectFieldFilter struct {
	name          string
	interfaceType interface{}
	typeVal       reflect.Type
}

func (r *ReflectFieldFilter) SetName(val string) {
	r.name = val
}

func (r *ReflectFieldFilter) SetInterfaceType(val interface{}) {
	r.interfaceType = val
}

func (r *ReflectFieldFilter) SetType(val interface{}) {
	r.typeVal = reflect.TypeOf(val)
}

type ReflectFieldFilterOpts interface {
	Apply(interface{})
}

func (t *TracerTools) ReflectGetValue(instance interface{}, filterOpts []interface{}) interface{} {
	instanceVal := reflect.ValueOf(instance)
	if instanceVal.Kind() == reflect.Ptr && instanceVal.Elem().Kind() == reflect.Struct {
		instanceVal = instanceVal.Elem()
	} else {
		// only support the pointer struct
		return nil
	}
	filter := &ReflectFieldFilter{}
	for _, opt := range filterOpts {
		if f, ok := opt.(ReflectFieldFilterOpts); ok {
			f.Apply(filter)
		}
	}
	for i := 0; i < instanceVal.NumField(); i++ {
		field := instanceVal.Field(i)
		fieldType := instanceVal.Type().Field(i)

		if t.checkFieldSupport(field, &fieldType, filter) {
			// for getting the export field value
			// 1. get a pointer to the field, then convert it to a 'generic' pointer
			// 2. convert the pointer back to an interface{}
			fieldPtr := unsafe.Pointer(field.UnsafeAddr())
			return reflect.NewAt(field.Type(), fieldPtr).Elem().Interface()
		}
	}
	return nil
}

func (t *TracerTools) checkFieldSupport(field reflect.Value, instanceField *reflect.StructField, filter *ReflectFieldFilter) bool {
	if filter.name != "" {
		if instanceField.Name != filter.name {
			return false
		}
	}
	if filter.interfaceType != nil {
		interfaceType := reflect.TypeOf(filter.interfaceType).Elem()
		if !field.Type().Implements(interfaceType) {
			return false
		}
	}
	if filter.typeVal != nil {
		if field.Type() != filter.typeVal {
			return false
		}
	}
	return true
}
