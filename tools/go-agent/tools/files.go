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
	"os"
	"path/filepath"
)

func WriteMultipleFile(baseDir string, nameWithData map[string]string) ([]string, error) {
	paths := make([]string, 0)
	for name, data := range nameWithData {
		fileName := filepath.Join(baseDir, name)
		if err := os.WriteFile(fileName, []byte(data), 0o600); err != nil {
			return nil, err
		}
		paths = append(paths, fileName)
	}

	return paths, nil
}

func WriteFile(baseDir, fileName, data string) (string, error) {
	res := filepath.Join(baseDir, fileName)
	if err := os.WriteFile(res, []byte(data), 0o600); err != nil {
		return "", err
	}
	return res, nil
}
