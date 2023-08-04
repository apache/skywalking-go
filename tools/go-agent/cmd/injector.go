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
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

const (
	projectBaseImportPath = "github.com/apache/skywalking-go"
	goModFileName         = "go.mod"

	swImportFileName = "skywalking_inject.go"
)

var (
	swImportFileContent = fmt.Sprintf(`// Code generated by skywalking-go-agent. DO NOT EDIT.

package main
	
import _ "%s"`, projectBaseImportPath)

	gitSHARegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$|^[0-9a-fA-F]{7}$`)
)

type projectInjector struct {
}

func InjectProject(flags *EnhancementToolFlags) error {
	stat, err := os.Stat(flags.Inject)
	if err != nil {
		return err
	}
	if version == "" {
		return fmt.Errorf("version is empty, please use the release version of skywalking-go")
	}
	abs, err := filepath.Abs(flags.Inject)
	if err != nil {
		return err
	}
	injector := &projectInjector{}
	if stat.IsDir() {
		return injector.injectDir(abs, flags.AllProjects)
	}
	return injector.injectFile(abs)
}

func (i *projectInjector) injectDir(path string, allProjects bool) error {
	if !i.findGoModFileInDir(path) {
		return fmt.Errorf("cannot fing go.mod file in %s, plase make sure that your inject path is a project directory", path)
	}
	// find all projects and main directory
	projects, err := i.findProjects(path, allProjects)
	if err != nil {
		return err
	}
	// filter validated projects
	validatedProjects := make([]*projectWithMainDirectory, 0)
	for _, project := range projects {
		if project.isValid() {
			validatedProjects = append(validatedProjects, project)
		}
	}
	fmt.Printf("total %d validate projects found\n", len(validatedProjects))
	// inject library
	for _, project := range validatedProjects {
		if err := i.injectLibraryInRoot(project.ProjectPath); err != nil {
			return err
		}
		for _, mainDir := range project.MainPackageDirs {
			contains, err := i.alreadyContainsLibraryImport(mainDir)
			if err != nil {
				return err
			}
			if contains {
				fmt.Printf("main package %s already contains imports, skip\n", mainDir)
				continue
			}

			// append a new file to the main package
			if err := i.appendNewImportFile(mainDir); err != nil {
				return fmt.Errorf("append new import file failed in %s, %v", mainDir, err)
			}
			fmt.Printf("append new import file success in %s\n", mainDir)
		}
	}
	return nil
}

func (i *projectInjector) injectFile(path string) error {
	if !strings.HasSuffix(path, ".go") {
		return fmt.Errorf("only support inject go file, %s is not a go file", path)
	}
	dir := filepath.Dir(path)
	if !i.findGoModFileInDir(dir) {
		return fmt.Errorf("cannot fing go.mod file in %s", dir)
	}
	// inject library
	if err := i.injectLibraryInRoot(dir); err != nil {
		return err
	}
	// if only inject to a file, then just add import into the file
	return i.injectImportInFile(path)
}

func (i *projectInjector) findGoModFileInDir(dir string) bool {
	path := filepath.Join(dir, goModFileName)
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat != nil
}

func (i *projectInjector) injectLibraryInRoot(dir string) error {
	v := version
	if !gitSHARegex.MatchString(version) {
		v = "v" + version
	}
	fmt.Printf("injecting skywalking-go@%s depenedency into %s\n", v, dir)
	command := exec.Command("go", "get", "github.com/apache/skywalking-go@"+v)
	command.Dir = dir
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err := command.Run()
	if err != nil {
		return err
	}
	return nil
}

func (i *projectInjector) injectImportInFile(path string) error {
	filename := filepath.Base(path)
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := decorator.ParseFile(nil, filename, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse file %s failed, %v", path, err)
	}
	if i.addingProjectImportInFileAndRewrite(f) {
		fmt.Printf("already existing library import in %s, skip\n", path)
	}
	fileContent, err := tools.GenerateDSTFileContent(f, nil)
	if err != nil {
		return fmt.Errorf("generate file content failed, %v", err)
	}
	err = os.WriteFile(path, []byte(fileContent), 0o600)
	if err != nil {
		return fmt.Errorf("rewrite the file %s failed, %v", path, err)
	}
	fmt.Printf("adding skywalking-go import into the file: %s", path)
	return nil
}

func (i *projectInjector) addingProjectImportInFileAndRewrite(f *dst.File) bool {
	var latestImportDel *dst.GenDecl
	var existingImport bool
	for _, decl := range f.Decls {
		if gen, ok := decl.(*dst.GenDecl); ok && gen != nil && gen.Tok == token.IMPORT {
			latestImportDel = gen
			if !existingImport && i.containsImport(gen) {
				existingImport = true
			}
		}
	}
	if existingImport {
		return true
	}
	if latestImportDel == nil {
		latestImportDel = &dst.GenDecl{
			Tok:   token.IMPORT,
			Specs: []dst.Spec{},
		}
		f.Decls = append([]dst.Decl{latestImportDel}, f.Decls...)
	}
	latestImportDel.Specs = append(latestImportDel.Specs, &dst.ImportSpec{
		Name: dst.NewIdent("_"),
		Path: &dst.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("%q", projectBaseImportPath),
		},
	})
	return false
}

func (i *projectInjector) findProjects(currentDir string, all bool) ([]*projectWithMainDirectory, error) {
	result := make([]*projectWithMainDirectory, 0)
	stack := make([]*projectWithMainDirectory, 0)
	currentStackPrefix := ""
	err := filepath.WalkDir(currentDir, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			return filepath.SkipDir
		}
		if currentStackPrefix != "" && !strings.HasPrefix(path, currentStackPrefix) {
			stack = stack[:len(stack)-1]
			currentStackPrefix = stack[len(stack)-1].ProjectPath
		}
		if f, e := os.Stat(filepath.Join(path, goModFileName)); e == nil && f != nil {
			if len(stack) > 0 && !all {
				return filepath.SkipDir
			}
			info := &projectWithMainDirectory{
				ProjectPath: path,
			}
			result = append(result, info)
			stack = append(stack, info)
			currentStackPrefix = path
		}
		if mainPackage, e := i.containsMainPackageInCurrentDirectory(path); e != nil {
			return err
		} else if mainPackage {
			currentModule := stack[len(stack)-1]
			currentModule.MainPackageDirs = append(currentModule.MainPackageDirs, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (i *projectInjector) containsMainPackageInCurrentDirectory(dir string) (bool, error) {
	readDir, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read dir %s failed, %v", dir, err)
	}
	for _, file := range readDir {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}

		parseFile, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, file.Name()), nil, parser.PackageClauseOnly)
		if err != nil {
			return false, err
		}
		if parseFile.Name.Name == "main" {
			return true, nil
		}

		// only needs to check the first .go file, other files should be same
		return false, nil
	}
	return false, nil
}

func (i *projectInjector) alreadyContainsLibraryImport(dir string) (bool, error) {
	readDir, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("reding directory %s failure, %v", dir, err)
	}
	for _, f := range readDir {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".go") {
			continue
		}
		file, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return false, fmt.Errorf("read file %s failed, %v", f.Name(), err)
		}

		dstFile, err := decorator.ParseFile(nil, f.Name(), file, parser.ImportsOnly)
		if err != nil {
			return false, fmt.Errorf("parsing file %s failed, %v", f.Name(), err)
		}

		var existingImport = false
		for _, decl := range dstFile.Decls {
			if gen, ok := decl.(*dst.GenDecl); ok && gen != nil && gen.Tok == token.IMPORT &&
				!existingImport && i.containsImport(gen) {
				existingImport = true
			}
		}
		if existingImport {
			return true, nil
		}
	}

	return false, nil
}

func (i *projectInjector) containsImport(imp *dst.GenDecl) bool {
	for _, spec := range imp.Specs {
		if i, ok := spec.(*dst.ImportSpec); !ok || i == nil {
			continue
		} else if i.Path != nil && i.Path.Value == fmt.Sprintf("%q", projectBaseImportPath) {
			return true
		}
	}
	return false
}

func (i *projectInjector) appendNewImportFile(dir string) error {
	importFilePath := filepath.Join(dir, swImportFileName)
	return os.WriteFile(importFilePath, []byte(swImportFileContent), 0o600)
}

type projectWithMainDirectory struct {
	ProjectPath     string
	MainPackageDirs []string
}

func (p *projectWithMainDirectory) isValid() bool {
	return p.ProjectPath != "" && len(p.MainPackageDirs) > 0
}
