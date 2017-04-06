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

package shared

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/stardog-union/service-broker/broker"
)

var (
	TESTURL_ENV = "CF_TESTING_STARDOG_URL"
)

type sdClientFactory struct {
	logger broker.SdLogger
}

func (cf *sdClientFactory) GetStardogAdminClient(sdURL string, dbCreds broker.DatabaseCredentials) broker.StardogClient {
	creds := broker.DatabaseCredentials{
		Password: "admin",
		Username: "admin",
	}
	c := broker.NewStardogClient(sdURL, creds, cf.logger)
	return c
}

func getClientFactory(planID string) (broker.Plan, bool, error) {
	sdURL := os.Getenv(TESTURL_ENV)
	if sdURL == "" {
		return nil, true, fmt.Errorf("The env %s must be set to run this test", TESTURL_ENV)
	}
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	sdLogger, _ := broker.NewSdLogger(baseLogger, "DEBUG")
	clientFactory := &sdClientFactory{logger: sdLogger}

	params := dataBasePlanFactory{
		StardogURL: sdURL,
		AdminName:  "admin",
		AdminPw:    "admin",
	}
	planFactory, err := GetPlanFactory(planID, params)
	if err != nil {
		return nil, false, fmt.Errorf("error getting the plan %s", err)
	}
	plan, err := planFactory.InflatePlan(&newDatabasePlanParameters{}, clientFactory, sdLogger)
	if err != nil {
		return nil, false, fmt.Errorf("error inflating the plan %s", err)
	}
	return plan, false, nil
}

func TestHappyPathSharedDbPlan(t *testing.T) {
	plan, skip, err := getClientFactory("aplan")
	if skip {
		t.Skipf(err.Error())
		return
	}

	code, serviceI, err := plan.CreateServiceInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusCreated {
		t.Fatalf("Create service returned an unsuccessful code %d", code)
	}

	bindParams, _ := json.Marshal(&newDatabaseBindParameters{})
	code, bindI, err := plan.Bind(serviceI, bindParams)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Bind returned an unsuccessful code %d", code)
	}

	code, err = plan.UnBind(bindI)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}
}

func TestSharedDbPlanRemoveTwice(t *testing.T) {
	plan, skip, err := getClientFactory("aplan")
	if skip {
		t.Skipf(err.Error())
		return
	}

	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusCreated {
		t.Fatalf("Create service returned an unsuccessful code %d", code)
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}
	code, _, err = plan.RemoveInstance()
	if err == nil {
		t.Fatalf("We should not have been able to delete twice")
		return
	}
	if code == http.StatusOK {
		t.Fatalf("The second remove should have failed")
	}
}

func TestSharedDbPlanInspectBind(t *testing.T) {
	plan, skip, err := getClientFactory("aplan")
	if skip {
		t.Skipf(err.Error())
		return
	}

	code, serviceI, err := plan.CreateServiceInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusCreated {
		t.Fatalf("Create service returned an unsuccessful code %d", code)
		return
	}
	s := serviceI.(serviceParameters)
	if s.DbName == "" {
		t.Fatalf("A db name should have been created")
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}
}

func TestSharedDbPlanConflictingDbName(t *testing.T) {
	plan, skip, err := getClientFactory("aplan")
	if skip {
		t.Skipf(err.Error())
		return
	}
	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusCreated {
		t.Fatalf("Create service returned an unsuccessful code %d", code)
		return
	}
	code, _, err = plan.CreateServiceInstance()
	if err == nil || code == http.StatusCreated {
		t.Fatalf("The second create should have failed")
		return
	}
	code, _, err = plan.RemoveInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}
}

func TestSharedDbPlanBindTwice(t *testing.T) {
	plan, skip, err := getClientFactory("aplan")
	if skip {
		t.Skipf(err.Error())
		return
	}

	code, serviceI, err := plan.CreateServiceInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusCreated {
		t.Fatalf("Create service returned an unsuccessful code %d", code)
	}

	bindParams, _ := json.Marshal(&newDatabaseBindParameters{
		Username: broker.GetRandomName("user", 5),
	})
	code, bindI, err := plan.Bind(serviceI, bindParams)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Bind returned an unsuccessful code %d", code)
	}
	bindParams, _ = json.Marshal(&newDatabaseBindParameters{
		Username: broker.GetRandomName("user", 5),
	})
	code, _, err = plan.Bind(serviceI, bindParams)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}

	code, err = plan.UnBind(bindI)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}
}

func TestSharedDbPlanUnbindTwice(t *testing.T) {
	plan, skip, err := getClientFactory("aplan")
	if skip {
		t.Skipf(err.Error())
		return
	}

	code, serviceI, err := plan.CreateServiceInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusCreated {
		t.Fatalf("Create service returned an unsuccessful code %d", code)
	}

	bindParams, _ := json.Marshal(&newDatabaseBindParameters{})
	code, bindI, err := plan.Bind(serviceI, bindParams)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Bind returned an unsuccessful code %d", code)
	}

	code, err = plan.UnBind(bindI)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}
	code, err = plan.UnBind(bindI)
	if err == nil || code == http.StatusOK {
		t.Fatalf("The second unbind should have failed")
		return
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	if code != http.StatusOK {
		t.Fatalf("Unbind returned an unsuccessful code %d", code)
	}
}
