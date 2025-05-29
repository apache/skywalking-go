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

package reporter

import (
	"html"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/apache/skywalking-go/plugins/core"
	"github.com/apache/skywalking-go/tools/go-agent/config"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/agentcore"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/api"
	"github.com/apache/skywalking-go/tools/go-agent/instrument/consts"
	"github.com/apache/skywalking-go/tools/go-agent/tools"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type Instrument struct {
	hasToEnhance bool
	compileOpts  *api.CompileOptions
}

func NewInstrument() *Instrument {
	return &Instrument{}
}

func (i *Instrument) CouldHandle(opts *api.CompileOptions) bool {
	i.compileOpts = opts
	return opts.Package == "github.com/apache/skywalking-go/agent/reporter"
}

func (i *Instrument) FilterAndEdit(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) bool {
	if i.hasToEnhance {
		return false
	}
	fileName := filepath.Base(path)
	reporterType := i.getReporterTypeConfig()
	if fileName == "imports.go" && reporterType == consts.GrpcReporter {
		tools.DeletePackageImports(curFile,
			"github.com/segmentio/kafka-go",
			"github.com/segmentio/kafka-go/compress",
			"google.golang.org/protobuf/proto")
		i.hasToEnhance = true
	}
	return true
}

func (i *Instrument) AfterEnhanceFile(fromPath, newPath string) error {
	return nil
}

func (i *Instrument) WriteExtraFiles(dir string) ([]string, error) {
	reporterType := i.getReporterTypeConfig()
	// copy reporter api files
	results := make([]string, 0)
	copiedFiles, err := tools.CopyGoFiles(core.FS, "reporter", dir, func(entry fs.DirEntry, f *dst.File) (*tools.DebugInfo, error) {
		if i.compileOpts.DebugDir == "" {
			return nil, nil
		}
		debugPath := filepath.Join(i.compileOpts.DebugDir, "plugins", "core", "reporter", entry.Name())
		return tools.BuildDSTDebugInfo(debugPath, nil)
	}, func(file *dst.File) {
		pkgUpdates := make(map[string]string)
		for _, p := range agentcore.CopiedSubPackages {
			key := strings.ReplaceAll(filepath.Join(agentcore.EnhanceFromBasePackage, p), `\`, `/`)
			val := strings.ReplaceAll(filepath.Join(agentcore.EnhanceBasePackage, p), `\`, `/`)
			pkgUpdates[key] = val
		}
		tools.ChangePackageImportPath(file, pkgUpdates)
	})
	if err != nil {
		return nil, err
	}
	results = append(results, copiedFiles...)

	copiedFiles, err = i.copyReporterFiles(dir, reporterType)
	if err != nil {
		return nil, err
	}
	results = append(results, copiedFiles...)

	// generate the file for export the reporter
	file, err := i.generateReporterInitFile(dir, reporterType)
	if err != nil {
		return nil, err
	}
	results = append(results, file)

	return results, nil
}

func (i *Instrument) getReporterTypeConfig() string {
	reporterType := config.GetConfig().Reporter.Type.GetStringResult()
	if reporterType == consts.KafkaReporter {
		return consts.KafkaReporter
	}
	return consts.GrpcReporter
}

// copy reporter implementations
// Force the use of '/' delimiter on all platforms
func (i *Instrument) copyReporterFiles(targetDir, reporterType string) ([]string, error) {
	copiedFilesResult := make([]string, 0)
	reporterDirName := strings.ReplaceAll(filepath.Join("reporter", reporterType), `\`, `/`)
	copiedFiles, err := tools.CopyGoFiles(core.FS, reporterDirName, targetDir, func(entry fs.DirEntry, f *dst.File) (*tools.DebugInfo, error) {
		if i.compileOpts.DebugDir == "" {
			return nil, nil
		}
		debugPath := filepath.Join(i.compileOpts.DebugDir, "plugins", "core", reporterDirName, entry.Name())
		return tools.BuildDSTDebugInfo(debugPath, f)
	}, func(file *dst.File) {
		file.Name = dst.NewIdent("reporter")
		pkgUpdates := make(map[string]string)
		for _, p := range agentcore.CopiedSubPackages {
			key := strings.ReplaceAll(filepath.Join(agentcore.EnhanceFromBasePackage, p), `\`, `/`)
			val := strings.ReplaceAll(filepath.Join(agentcore.EnhanceBasePackage, p), `\`, `/`)
			pkgUpdates[key] = val
		}
		tools.ChangePackageImportPath(file, pkgUpdates)
		tools.DeletePackageImports(file, "github.com/apache/skywalking-go/plugins/core/reporter")
	})

	if err != nil {
		return nil, err
	}
	copiedFilesResult = append(copiedFilesResult, copiedFiles...)
	return copiedFilesResult, nil
}

func (i *Instrument) generateReporterInitFile(dir, reporterType string) (string, error) {
	reporterInitTemplate := baseReporterInitTemplate
	if reporterType == consts.KafkaReporter {
		reporterInitTemplate += `
	_, cdsManager, err := initManager(logger, checkInterval)
	if err != nil {
		return nil, err
	}
	return initKafkaReporter(logger, checkInterval, cdsManager)
}`
		reporterInitTemplate += kafkaReporterInitFunc
	} else {
		reporterInitTemplate += `
	connManager, cdsManager, err := initManager(logger, checkInterval)
	if err != nil {
		return nil, err
	}
	return initGRPCReporter(logger, checkInterval, connManager, cdsManager)
}`
		reporterInitTemplate += grpcReporterInitFunc
	}
	reporterInitTemplate += initManagerFunc
	return tools.WriteFile(dir, "reporter_init.go", html.UnescapeString(tools.ExecuteTemplate(reporterInitTemplate, struct {
		InitFuncName string
		Config       *config.Config
	}{
		InitFuncName: consts.ReporterInitFuncName,
		Config:       config.GetConfig(),
	})))
}

const baseReporterInitTemplate = `package reporter

import (
	"github.com/apache/skywalking-go/agent/core/operator"
	"fmt"
	"strconv"
	"os"
	"time"
	"strings"
)

func {{.InitFuncName}}(logger operator.LogOperator) (Reporter, error) {
	if {{.Config.Reporter.Discard.ToGoBoolValue}} {
		return NewDiscardReporter(), nil
	}
	checkIntervalVal := {{.Config.Reporter.CheckInterval.ToGoIntValue "the reporter check interval must be number"}}
	checkInterval := time.Second * time.Duration(checkIntervalVal)
`

const initManagerFunc = `

func initManager(logger operator.LogOperator, checkInterval time.Duration) (*ConnectionManager, *CDSManager, error) {
	authenticationVal := {{.Config.Reporter.GRPC.Authentication.ToGoStringValue}}
	backendServiceVal := {{.Config.Reporter.GRPC.BackendService.ToGoStringValue}}

	var (
		connManager *ConnectionManager
		err        error
	)
	if {{.Config.Reporter.GRPC.TLS.Enable.ToGoBoolValue}} {
		tc, err := generateTLSCredential({{.Config.Reporter.GRPC.TLS.CAPath.ToGoStringValue}}, 
			{{.Config.Reporter.GRPC.TLS.ClientKeyPath.ToGoStringValue}},
			{{.Config.Reporter.GRPC.TLS.ClientCertChainPath.ToGoStringValue}},
			{{.Config.Reporter.GRPC.TLS.InsecureSkipVerify.ToGoBoolValue}})
		if err != nil {
			panic(fmt.Sprintf("generate go agent tls credential error: %v", err))
		}
		connManager, err = NewConnectionManager(logger, checkInterval, backendServiceVal, authenticationVal, tc)
	} else {
		connManager, err = NewConnectionManager(logger, checkInterval, backendServiceVal, authenticationVal, nil)
	}
	if err != nil {
		return nil, nil, err
	}

	cdsFetchIntervalVal := {{.Config.Reporter.GRPC.CDSFetchInterval.ToGoIntValue "the cds fetch interval must be number"}}
	cdsFetchInterval := time.Second * time.Duration(cdsFetchIntervalVal)
	cdsManager, err := NewCDSManager(logger, backendServiceVal, cdsFetchInterval, connManager)
	if err != nil {
		return nil, nil, err
	}
	return connManager, cdsManager, nil
}
`

const grpcReporterInitFunc = `

func initGRPCReporter(logger operator.LogOperator,
					checkInterval time.Duration,
					connManager *ConnectionManager,
					cdsManager *CDSManager) (Reporter, error) {
	var opts []ReporterOption
	maxSendQueueVal := {{.Config.Reporter.GRPC.MaxSendQueue.ToGoIntValue "the GRPC reporter max queue size must be number"}}
	opts = append(opts, WithMaxSendQueueSize(maxSendQueueVal))

	backendServiceVal := {{.Config.Reporter.GRPC.BackendService.ToGoStringValue}}
	return NewGRPCReporter(logger, backendServiceVal, checkInterval, connManager, cdsManager, opts...)
}
`

const kafkaReporterInitFunc = `

func initKafkaReporter(logger operator.LogOperator, checkInterval time.Duration, cdsManager *CDSManager) (Reporter, error) {
    var opts []ReporterOptionKafka

    topicSegment := {{.Config.Reporter.Kafka.TopicSegment.ToGoStringValue}}
	opts = append(opts, WithKafkaTopicSegment(topicSegment))
	topicMeter := {{.Config.Reporter.Kafka.TopicMeter.ToGoStringValue}}
	opts = append(opts, WithKafkaTopicMeter(topicMeter))
	topicLogging := {{.Config.Reporter.Kafka.TopicLogging.ToGoStringValue}}
	opts = append(opts, WithKafkaTopicLogging(topicLogging))
	topicManagement := {{.Config.Reporter.Kafka.TopicManagement.ToGoStringValue}}
	opts = append(opts, WithKafkaTopicManagement(topicManagement))

    maxSendQueueVal := {{.Config.Reporter.Kafka.MaxSendQueue.ToGoIntValue "the Kafka reporter max queue size must be a number"}}
    opts = append(opts, WithKafkaMaxSendQueueSize(maxSendQueueVal))
    batchSizeVal := {{.Config.Reporter.Kafka.BatchSize.ToGoIntValue "the Kafka reporter batch size must be a number"}}
    opts = append(opts, WithKafkaBatchSize(batchSizeVal))
    batchBytesVal := {{.Config.Reporter.Kafka.BatchBytes.ToGoIntValue "the Kafka reporter batch bytes must be a number"}}
    opts = append(opts, WithKafkaBatchBytes(int64(batchBytesVal)))
    batchTimeoutMillisVal := {{.Config.Reporter.Kafka.BatchTimeoutMillis.ToGoIntValue "the Kafka reporter batch timeout must be a number"}}
    opts = append(opts, WithKafkaBatchTimeoutMillis(batchTimeoutMillisVal))
    acksVal := {{.Config.Reporter.Kafka.Acks.ToGoIntValue "the Kafka reporter acks must be a number"}}
    opts = append(opts, WithKafkaAcks(acksVal))
    
	brokers := {{.Config.Reporter.Kafka.Brokers.ToGoStringValue}}
    return NewKafkaReporter(logger, brokers, checkInterval, cdsManager, opts...)
}
`
