//
//  Copyright (c) 2017, Stardog Union. <http://stardog.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/stardog-union/service-broker/broker"
	"github.com/stardog-union/service-broker/plans/perinstance"
	"github.com/stardog-union/service-broker/plans/shared"
	_ "github.com/stardog-union/service-broker/store/sql"
	storesql "github.com/stardog-union/service-broker/store/sql"
	storestardog "github.com/stardog-union/service-broker/store/stardog"
)

func handlePlugins(conf *broker.ServerConfig) (map[string]broker.PlanFactory, error) {
	databasePlanMap := make(map[string]broker.PlanFactory)

	for _, plan := range conf.Plans {
		if plan.PlanName == "shared_database_plan" {
			sharedDbPlan, err := shared.GetPlanFactory(plan.PlanID, plan.Parameters)
			if err != nil {
				return nil, err
			}
			databasePlanMap[plan.PlanID] = sharedDbPlan
		} else if plan.PlanName == "perinstance" {
			perinstancePlan, err := perinstance.GetPlanFactory(plan.PlanID, plan.Parameters)
			if err != nil {
				return nil, err
			}
			databasePlanMap[plan.PlanID] = perinstancePlan
		} else {
			return nil, fmt.Errorf("No plan named %s exists", plan.PlanName)
		}
	}

	return databasePlanMap, nil
}

func main() {
	var conf broker.ServerConfig

	confPath := filepath.Join("data", "conf.json")
	if len(os.Args) > 1 {

		confPath = strings.TrimSpace(os.Args[1])
	}
	err := broker.LoadJSON(&conf, confPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with configuration %s\n", err)
		os.Exit(1)
	}
	cfPort := os.Getenv("PORT")
	if cfPort != "" {
		conf.Port = cfPort
	}
	username := os.Getenv("SECURITY_USER_NAME")
	if username != "" {
		conf.BrokerUsername = username
	}
	pw := os.Getenv("SECURITY_USER_PASSWORD")
	if pw != "" {
		conf.BrokerPassword = pw
	}

	logFd := os.Stderr
	if conf.LogFile != "" {
		f, err := os.OpenFile(conf.LogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open the logging file.  Logging to stderr: %s\n", err)
		} else {
			logFd = f
		}
	}

	baseLogger := log.New(logFd, "", log.Ldate|log.Ltime)
	logger, err := broker.NewSdLogger(baseLogger, conf.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed create the logger %s\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "USING PORT: %s\n", conf.Port)

	planMap, err := handlePlugins(&conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing the configuration: %s\n", err)
		os.Exit(2)
	}
	var store broker.Store
	if conf.Storage.Type == "stardog" {
		store, err = storestardog.NewStardogStore(conf.BrokerID, logger, conf.Storage.Parameters)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up the data store: %s\n", err)
			os.Exit(3)
		}
	} else if conf.Storage.Type == "sql" {
		store, err = storesql.NewMySQLStore(conf.BrokerID, logger, conf.Storage.Parameters)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up the data store: %s\n", err)
			os.Exit(4)
		}
	} else {
		fmt.Fprintf(os.Stderr, "The datastore %s is not supported.\n", conf.Storage.Type)
		os.Exit(5)
	}

	clientFactory := broker.NewClientFactory(logger)
	s, err := broker.CreateServer(planMap, &conf, clientFactory, logger, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting the server: %s\n", err)
		os.Exit(4)
	}
	err = s.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting the server: %s\n", err)
		os.Exit(5)
	}
	err = s.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error waiting on the server: %s\n", err)
		os.Exit(5)
	}
}
