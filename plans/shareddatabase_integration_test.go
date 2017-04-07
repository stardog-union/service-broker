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

package plans

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/stardog-union/service-broker/broker"
	"github.com/stardog-union/service-broker/plans/perinstance"
	"github.com/stardog-union/service-broker/plans/shared"
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

type planTester interface {
	GetCreateParameters() interface{}
	GetBindParameters() interface{}
	GetPlanFactory(sdURL string, planID string) (broker.Plan, error)
}

type planTestFunc func(planTester) (bool, error)

type sharedDbTester struct {
}

func (sdb *sharedDbTester) GetCreateParameters() interface{} {
	return nil
}

func (sdb *sharedDbTester) GetBindParameters() interface{} {
	return nil
}

func (sdb *sharedDbTester) GetPlanFactory(sdURL string, planID string) (broker.Plan, error) {
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	sdLogger, _ := broker.NewSdLogger(baseLogger, "DEBUG")
	clientFactory := &sdClientFactory{logger: sdLogger}

	params := make(map[string]string)
	params["stardog_url"] = sdURL
	params["admin_username"] = "admin"
	params["admin_password"] = "admin"

	planFactory, err := shared.GetPlanFactory(planID, params)
	if err != nil {
		return nil, fmt.Errorf("error getting the plan %s", err)
	}
	plan, err := planFactory.InflatePlan(sdb.GetCreateParameters(), clientFactory, sdLogger)
	if err != nil {
		return nil, fmt.Errorf("error inflating the plan %s", err)
	}
	return plan, nil
}

type instanceDbTester struct {
	params map[string]string
}

func (i *instanceDbTester) GetCreateParameters() interface{} {
	return i.params
}

func (i *instanceDbTester) GetBindParameters() interface{} {
	return nil
}

func (i *instanceDbTester) GetPlanFactory(sdURL string, planID string) (broker.Plan, error) {
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	sdLogger, _ := broker.NewSdLogger(baseLogger, "DEBUG")
	clientFactory := &sdClientFactory{logger: sdLogger}

	params := make(map[string]string)
	params["url"] = sdURL
	params["username"] = "admin"
	params["password"] = "admin"
	i.params = params

	planFactory, err := perinstance.GetPlanFactory(planID, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting the plan %s", err)
	}
	plan, err := planFactory.InflatePlan(i.GetCreateParameters(), clientFactory, sdLogger)
	if err != nil {
		return nil, fmt.Errorf("error inflating the plan %s", err)
	}
	return plan, nil
}

func getClientFactory(planID string, tester planTester) (broker.Plan, bool, error) {
	sdURL := os.Getenv(TESTURL_ENV)
	if sdURL == "" {
		return nil, true, fmt.Errorf("The env %s must be set to run this test", TESTURL_ENV)
	}
	p, err := tester.GetPlanFactory(sdURL, planID)
	return p, false, err
}

func happyPathSharedDbPlan(tester planTester) (bool, error) {
	plan, skip, err := getClientFactory("aplan", tester)
	if skip {
		return true, nil
	}

	fmt.Printf("pre CreateServiceInstance")
	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusCreated {
		return false, fmt.Errorf("Create service returned an unsuccessful code %d", code)
	}

	fmt.Printf("pre GetBindParameters")
	bindParams := tester.GetBindParameters()
	code, bindI, err := plan.Bind(bindParams)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Bind returned an unsuccessful code %d", code)
	}

	code, err = plan.UnBind(bindI)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}
	return false, nil
}

func TestHappyPathSharedDbPlan(t *testing.T) {
	skip, err := happyPathSharedDbPlan(&sharedDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func TestHappyPathInstanceDbPlan(t *testing.T) {
	skip, err := happyPathSharedDbPlan(&instanceDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func sharedDbPlanRemoveTwice(tester planTester) (bool, error) {
	plan, skip, err := getClientFactory("aplan", tester)
	if skip {
		return true, nil
	}

	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusCreated {
		return false, fmt.Errorf("Create service returned an unsuccessful code %d", code)
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		return false, fmt.Errorf(err.Error())
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}
	code, _, err = plan.RemoveInstance()
	if err == nil {
		return false, fmt.Errorf("We should not have been able to delete twice")
	}
	if code == http.StatusOK {
		return false, fmt.Errorf("The second remove should have failed")
	}
	return false, nil
}

func TestSharedDbPlanRemoveTwice(t *testing.T) {
	skip, err := sharedDbPlanRemoveTwice(&sharedDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func TestSharedPerInstanceRemoveTwice(t *testing.T) {
	skip, err := sharedDbPlanRemoveTwice(&instanceDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func sharedDbPlanInspectBind(tester planTester) (bool, error) {
	plan, skip, err := getClientFactory("aplan", tester)
	if skip {
		return true, nil
	}

	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusCreated {
		return false, fmt.Errorf("Create service returned an unsuccessful code %d", code)
	}
	// XXX TODO figure out how to inspect this stuff at this level.  probably doesn't belong here
	// s := serviceI.(serviceParameters)
	// if s.DbName == "" {
	// 	return false, fmt.Errorf("A db name should have been created")
	// }

	code, _, err = plan.RemoveInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}
	return false, nil
}

func TestSharedDbPlanInspectBind(t *testing.T) {
	skip, err := sharedDbPlanInspectBind(&sharedDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func TestInstancePlanInspectBind(t *testing.T) {
	skip, err := sharedDbPlanInspectBind(&instanceDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func sharedDbPlanConflictingDbName(tester planTester) (bool, error) {
	plan, skip, err := getClientFactory("aplan", tester)
	if skip {
		return true, nil
	}

	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusCreated {
		return false, fmt.Errorf("Create service returned an unsuccessful code %d", code)
	}
	code, _, err = plan.CreateServiceInstance()
	if err == nil || code == http.StatusCreated {
		return false, fmt.Errorf("The second create should have failed")
	}
	code, _, err = plan.RemoveInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}
	return false, nil
}

func TestSharedDbPlanConflictingDbName(t *testing.T) {
	skip, err := sharedDbPlanConflictingDbName(&sharedDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func TestInstancePlanConflictingDbName(t *testing.T) {
	skip, err := sharedDbPlanConflictingDbName(&instanceDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func sharedDbPlanBindTwice(tester planTester) (bool, error) {
	plan, skip, err := getClientFactory("aplan", tester)
	if skip {
		return true, nil
	}

	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusCreated {
		return false, fmt.Errorf("Create service returned an unsuccessful code %d", code)
	}

	bindParams := tester.GetBindParameters()
	code, bindI, err := plan.Bind(bindParams)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Bind returned an unsuccessful code %d", code)
	}
	bindParams = tester.GetBindParameters()
	code, _, err = plan.Bind(bindParams)
	if err != nil {
		return false, err
	}

	code, err = plan.UnBind(bindI)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}
	return false, nil
}

func TestSharedDbPlanBindTwice(t *testing.T) {
	skip, err := sharedDbPlanBindTwice(&sharedDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func TestPerinstancePlanBindTwice(t *testing.T) {
	skip, err := sharedDbPlanBindTwice(&instanceDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func sharedDbPlanUnbindTwice(tester planTester) (bool, error) {
	plan, skip, err := getClientFactory("aplan", tester)
	if skip {
		return true, nil
	}

	code, _, err := plan.CreateServiceInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusCreated {
		return false, fmt.Errorf("Create service returned an unsuccessful code %d", code)
	}

	bindParams := tester.GetBindParameters()
	code, bindI, err := plan.Bind(bindParams)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Bind returned an unsuccessful code %d", code)
	}

	code, err = plan.UnBind(bindI)
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}
	code, err = plan.UnBind(bindI)
	if err == nil || code == http.StatusOK {
		return false, fmt.Errorf("The second unbind should have failed")
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		return false, err
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("Unbind returned an unsuccessful code %d", code)
	}
	return false, nil
}

func TestSharedDbPlanUnbindTwice(t *testing.T) {
	skip, err := sharedDbPlanUnbindTwice(&sharedDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}

func TestInstancePlanUnbindTwice(t *testing.T) {
	skip, err := sharedDbPlanUnbindTwice(&instanceDbTester{})
	if skip {
		t.Skip("Skipping the test")
	}
	if err != nil {
		t.Fatalf("The test failed %s", err)
	}
}
