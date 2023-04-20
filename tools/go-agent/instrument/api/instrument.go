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

package api

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type Instrument interface {
	// CouldHandle verify current instrument can handle this request
	CouldHandle(opts *CompileOptions) bool
	// FilterAndEdit filter the matched data which decode by DST, and edit the data
	FilterAndEdit(path string, cursor *dstutil.Cursor, allFiles []*dst.File) bool
	// AfterEnhanceFile after the enhanced file been written, check the file is needs rewrite
	AfterEnhanceFile(fromPath, newPath string) error
	// WriteExtraFiles customized the extra files when there have instrumented files
	WriteExtraFiles(dir string) ([]string, error)
}
