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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsLibraryImport(t *testing.T) {
	injector := &projectInjector{}
	libraryImport, err := injector.alreadyContainsLibraryImport("./testdata/noimports")
	assert.Nil(t, err, "should not return an error")
	assert.False(t, libraryImport, "should not contain library import")
	libraryImport, err = injector.alreadyContainsLibraryImport("./testdata/imports")
	assert.Nil(t, err, "should not return an error")
	assert.True(t, libraryImport, "should contain library import")
}

func TestContainsMainPackage(t *testing.T) {
	injector := &projectInjector{}
	mainPackage, err := injector.containsMainPackageInCurrentDirectory("./testdata/entry")
	assert.Nil(t, err, "should not return an error")
	assert.True(t, mainPackage, "should contain main package")
}

func TestGithubSHA(t *testing.T) {
	assert.True(t, gitSHARegex.MatchString("f7a33a6d91a74a3e8b524f9395b0457ea64c02b8"), "should be GitHub SHA")
	assert.True(t, gitSHARegex.MatchString("f7a33a6"), "should be GitHub short SHA")
	assert.False(t, gitSHARegex.MatchString("0.1.0"), "should not be GitHub SHA")
}

func TestGetCompitableVersion(t *testing.T) {
	assert.Equal(t, "v0.1.0", getCompitableVersion("v0.1.0"))
	assert.Equal(t, "v0.1.0", getCompitableVersion("0.1.0"))
}

func TestProjectWithMainDirectory(t *testing.T) {
	assert.True(t, (&projectWithMainDirectory{
		ProjectPath:     "./testdata/entry",
		MainPackageDirs: []string{"./testdata/entry"},
	}).isValid())
	assert.False(t, (&projectWithMainDirectory{
		MainPackageDirs: []string{"./testdata/entry"},
	}).isValid())
	assert.False(t, (&projectWithMainDirectory{
		ProjectPath: "./testdata/entry",
	}).isValid())
}
