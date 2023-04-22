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

package rewrite

import (
	"fmt"
	"strings"

	"github.com/dave/dst"
)

func (c *Context) Var(val *dst.ValueSpec) {
	if len(val.Names) == 1 {
		oldName := val.Names[0].Name
		if !strings.HasPrefix(oldName, GenerateVarPrefix) {
			val.Names[0] = dst.NewIdent(fmt.Sprintf("%s%s%s", VarPrefix, c.currentPackageTitle, oldName))
			c.rewriteMapping.addVarMapping(oldName, val.Names[0].Name)
		}
	}
	c.enhanceTypeNameWhenRewrite(val.Type, val, -1)
	for _, subVal := range val.Values {
		c.enhanceTypeNameWhenRewrite(subVal, val, -1)
	}
}
