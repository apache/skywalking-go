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

package instrument

import (
	"go/parser"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"

	"github.com/sirupsen/logrus"

	"github.com/apache/skywalking-go/tools/go-agent/instrument/agentcore"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/entry"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/goroutine"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/logger"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/reporter"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/runtime"
	"github.com/apache/skywalking-go/tools/go-agent/tools"
)

var instruments = []api.Instrument{
	runtime.NewInstrument(),
	agentcore.NewInstrument(),
	reporter.NewInstrument(),
	entry.NewInstrument(),
	logger.NewInstrument(),
	// Insert goroutine wrapper instrument to ensure labels at goroutine start
	goroutine.NewInstrument(),
	plugins.NewInstrument(),
}

func Execute(opts *api.CompileOptions, args []string) ([]string, error) {
	// if the options is invalid, just ignore
	if !opts.IsValid() {
		return args, nil
	}

	// remove the vendor directory to get the real package name
	opts.Package = tools.UnVendor(opts.Package)

	// init the logger for the instrument
	loggerFile, err := initLogger(opts)
	if err != nil {
		return nil, err
	}
	defer loggerFile.Close()
	logrus.Infof("executing instrument with args: %v", args)

	return execute0(opts, args)
}

func execute0(opts *api.CompileOptions, args []string) ([]string, error) {
	// find all matching instruments and execute all, preserving order
	matching := make([]api.Instrument, 0)
	for _, ins := range instruments {
		if ins.CouldHandle(opts) {
			matching = append(matching, ins)
		}
	}
	if len(matching) == 0 {
		return args, nil
	}

	buildDir := filepath.Dir(opts.Output)

	for _, inst := range matching {
		// instrument existing files
		if err := instrumentFiles(buildDir, inst, args); err != nil {
			return nil, err
		}

		// write extra files if exist
		files, err := inst.WriteExtraFiles(buildDir)
		if err != nil {
			return nil, err
		}
		if len(files) > 0 {
			args = append(args, files...)
		}
	}

	return args, nil
}

func instrumentFiles(buildDir string, inst api.Instrument, args []string) error {
	// parse files
	parsedFiles, err := parseFilesInArgs(args)
	if err != nil {
		return err
	}

	allFiles := make([]*dst.File, 0)
	for _, f := range parsedFiles {
		allFiles = append(allFiles, f.dstFile)
	}

	// filter and edit the files
	instrumentedFiles := make([]string, 0)
	for path, info := range parsedFiles {
		hasInstruted := false
		dstutil.Apply(info.dstFile, func(cursor *dstutil.Cursor) bool {
			if inst.FilterAndEdit(path, info.dstFile, cursor, allFiles) {
				hasInstruted = true
			}
			return true
		}, func(cursor *dstutil.Cursor) bool {
			return true
		})

		if hasInstruted {
			instrumentedFiles = append(instrumentedFiles, path)
		}
	}

	// write instrumented files to the build directory
	for _, updateFileSrc := range instrumentedFiles {
		info := parsedFiles[updateFileSrc]
		filename := filepath.Base(updateFileSrc)
		dest := strings.ReplaceAll(filepath.Join(buildDir, filename), `\`, `/`)
		debugInfo, err := tools.BuildDSTDebugInfo(updateFileSrc, nil)
		if err != nil {
			return err
		}
		if err := tools.WriteDSTFile(dest, info.dstFile, debugInfo); err != nil {
			return err
		}
		if err := inst.AfterEnhanceFile(updateFileSrc, dest); err != nil {
			return err
		}
		args[info.argsIndex] = dest
	}

	return nil
}

func parseFilesInArgs(args []string) (map[string]*fileInfo, error) {
	parsedFiles := make(map[string]*fileInfo)
	var lastPath string
	defer func() {
		if e := recover(); e != nil {
			logrus.Errorf("panic when parsing files: %s: %v", lastPath, e)
		}
	}()
	for inx, path := range args {
		// only process the go file
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		lastPath = path

		// parse the file
		file, err := decorator.ParseFile(nil, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}

		parsedFiles[path] = &fileInfo{
			argsIndex: inx,
			dstFile:   file,
		}
	}

	return parsedFiles, nil
}

func initLogger(opts *api.CompileOptions) (*os.File, error) {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	file, err := os.OpenFile(filepath.Join(opts.CompileBaseDir(), "instrument.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
	if err != nil {
		return nil, err
	}
	logrus.SetOutput(file)

	return file, nil
}

type fileInfo struct {
	argsIndex int
	dstFile   *dst.File
}
