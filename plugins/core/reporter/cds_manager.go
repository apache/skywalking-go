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
	"context"
	"time"

	"github.com/apache/skywalking-go/plugins/core/operator"

	configuration "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
)

type CDSManager struct {
	logger      operator.LogOperator
	serverAddr  string
	cdsInterval time.Duration
	cdsService  *ConfigDiscoveryService
	cdsClient   configuration.ConfigurationDiscoveryServiceClient
	connManager *ConnectionManager
	entity      *Entity
}

func NewCDSManager(logger operator.LogOperator, serverAddr string, cdsInterval time.Duration, connManager *ConnectionManager) (*CDSManager, error) {
	cds := &CDSManager{
		logger:      logger,
		serverAddr:  serverAddr,
		cdsInterval: cdsInterval,
		connManager: connManager,
	}
	if cdsInterval > 0 {
		conn, err := connManager.GetConnection(serverAddr)
		if err != nil {
			return nil, err
		}
		cds.cdsClient = configuration.NewConfigurationDiscoveryServiceClient(conn)
		cds.cdsService = NewConfigDiscoveryService()
	}
	return cds, nil
}

func (r *CDSManager) InitCDS(entity *Entity, cdsWatchers []AgentConfigChangeWatcher) {
	if r.cdsClient == nil {
		return
	}
	r.entity = entity

	// bind watchers
	r.cdsService.BindWatchers(cdsWatchers)

	// fetch config
	go func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("CDSManager InitCDS panic err %v", err)
			}
		}()
		for {
			switch r.connManager.GetConnectionStatus(r.serverAddr) {
			case ConnectionStatusShutdown:
				break
			case ConnectionStatusDisconnect:
				time.Sleep(r.cdsInterval)
				continue
			}

			configurations, err := r.cdsClient.FetchConfigurations(context.Background(), &configuration.ConfigurationSyncRequest{
				Service: r.entity.ServiceName,
				Uuid:    r.cdsService.UUID,
			})

			if err != nil {
				r.logger.Errorf("fetch dynamic configuration error %v", err)
				time.Sleep(r.cdsInterval)
				continue
			}
			r.logger.Infof("configurations: %+v, len: %d", configurations, len(configurations.GetCommands()))

			if len(configurations.GetCommands()) > 0 && configurations.GetCommands()[0].Command == "ConfigurationDiscoveryCommand" {
				command := configurations.GetCommands()[0]
				r.cdsService.HandleCommand(command)
			}

			time.Sleep(r.cdsInterval)
		}
	}()
}
