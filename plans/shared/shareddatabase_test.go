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

func getLogger() (broker.SdLogger, error) {
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	return broker.NewSdLogger(baseLogger, "DEBUG")
}

type fakeClientFactory struct {
	createDb   []fakeClientCommands
	deleteDb   []fakeClientCommands
	userExists []fakeClientCommands
	newUser    []fakeClientCommands
	deleteUser []fakeClientCommands
	grantUser  []fakeClientCommands
	revokeUser []fakeClientCommands

	failures          map[string]bool
	userExistResponse bool
}

func createFakeClientFactory(userExistsResponse bool, failures ...string) *fakeClientFactory {
	cf := fakeClientFactory{}
	cf.createDb = make([]fakeClientCommands, 0, 10)
	cf.deleteDb = make([]fakeClientCommands, 0, 10)
	cf.userExists = make([]fakeClientCommands, 0, 10)
	cf.newUser = make([]fakeClientCommands, 0, 10)
	cf.deleteUser = make([]fakeClientCommands, 0, 10)
	cf.grantUser = make([]fakeClientCommands, 0, 10)
	cf.revokeUser = make([]fakeClientCommands, 0, 10)
	cf.failures = make(map[string]bool)
	cf.userExistResponse = userExistsResponse
	for _, f := range failures {
		cf.failures[f] = true
	}
	return &cf
}

func (c *fakeClientFactory) GetStardogAdminClient(sdURL string, dbCreds broker.DatabaseCredentials) broker.StardogClient {
	return &fakeClient{factory: c}
}

type fakeClientCommands struct {
	dbName   string
	username string
	pw       string
}

type fakeClient struct {
	factory *fakeClientFactory
}

func (c *fakeClient) CreateDatabase(dbName string) error {
	c.factory.createDb = append(c.factory.createDb, fakeClientCommands{dbName: dbName})
	if c.factory.failures["CreateDatabase"] {
		return fmt.Errorf("Mock test forced error")
	}
	return nil
}

func (c *fakeClient) DeleteDatabase(dbName string) error {
	c.factory.deleteDb = append(c.factory.deleteDb, fakeClientCommands{dbName: dbName})
	if c.factory.failures["DeleteDatabase"] {
		return fmt.Errorf("Mock test forced error")
	}
	return nil
}

func (c *fakeClient) UserExists(username string) (bool, error) {
	c.factory.userExists = append(c.factory.userExists, fakeClientCommands{username: username})
	if c.factory.failures["UserExists"] {
		return false, fmt.Errorf("Mock test forced error")
	}
	return c.factory.userExistResponse, nil
}

func (c *fakeClient) NewUser(username string, pw string) error {
	c.factory.newUser = append(c.factory.newUser, fakeClientCommands{username: username, pw: pw})
	if c.factory.failures["NewUser"] {
		return fmt.Errorf("Mock test forced error")
	}
	return nil
}

func (c *fakeClient) DeleteUser(username string) error {
	c.factory.deleteUser = append(c.factory.deleteUser, fakeClientCommands{username: username})
	if c.factory.failures["DeleteUser"] {
		return fmt.Errorf("Mock test forced error")
	}
	return nil
}

func (c *fakeClient) GrantUserAccessToDb(dbName string, username string) error {
	c.factory.grantUser = append(c.factory.grantUser, fakeClientCommands{dbName: dbName, username: username})
	if c.factory.failures["GrantUserAccessToDb"] {
		return fmt.Errorf("Mock test forced error")
	}
	return nil
}

func (c *fakeClient) RevokeUserAccess(dbName string, username string) error {
	c.factory.revokeUser = append(c.factory.revokeUser, fakeClientCommands{dbName: dbName, username: username})
	if c.factory.failures["RevokeUserAccess"] {
		return fmt.Errorf("Mock test forced error")
	}
	return nil
}

func TestSimpleUnitSharedDbPlan(t *testing.T) {
	sdURL := "http://notreal.fake:5820"
	dbFactory := dataBasePlanFactory{
		StardogURL: sdURL,
		AdminName:  "admin",
		AdminPw:    "admin",
	}
	planID := "aplanid"
	planFactory, err := GetPlanFactory(planID, dbFactory)
	if err != nil {
		fmt.Printf("Failed to get the factory %s\n", err)
		t.Fatal()
		return
	}

	dbName := "aDbName"
	logger, err := getLogger()
	clientFactory := createFakeClientFactory(false)
	if planFactory.PlanName() != "shareddb" {
		t.Fatalf("The plan name was incorrect\n")
		return
	}
	if !planFactory.Bindable() {
		t.Fatalf("The plan should be bindable\n")
		return
	}
	if !planFactory.Free() {
		t.Fatalf("The plan should be free\n")
		return
	}
	if planFactory.Metadata() != nil {
		t.Fatalf("The metadata should be nil\n")
		return
	}
	if planFactory.PlanID() != planID {
		t.Fatalf("The plan id was not correct\n")
		return
	}
	if planFactory.PlanDescription() == "" {
		t.Fatalf("The plan description should not be null\n")
		return
	}
	plan := planFactory.MakePlan(clientFactory, logger)
	if plan.PlanID() != planID {
		t.Fatalf("The plan id was not correct from the plan\n")
		return
	}

	params := newDatabasePlanParameters{DbName: dbName}
	paramsBytes, err := json.Marshal(&params)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %s\n", err)
		return
	}
	code, dataI, err := plan.CreateServiceInstance(paramsBytes)
	if code != http.StatusCreated {
		t.Fatalf("The status should be ok %s\n", err)
		return
	}

	if len(clientFactory.createDb) != 1 {
		t.Fatalf("Create database was not called on the client")
		return
	}
	if clientFactory.createDb[0].dbName != dbName {
		t.Fatalf("The wrong db name was used")
		return
	}

	data := dataI.(serviceParameters)
	if data.DbName != dbName {
		t.Fatalf("The db name was not correct %s", err)
		return
	}

	username := "someuser"
	password := "somepw"
	dbParams := newDatabaseBindParameters{
		Username: username,
		Password: password,
	}
	b, err := json.Marshal(&dbParams)
	if err != nil {
		t.Fatalf("failed to marshall the parameters %s", err)
		return
	}

	code, bindDataI, err := plan.Bind(dataI, b)
	if code != http.StatusOK {
		t.Fatalf("The status should be ok after bind %s", err)
		return
	}

	bindData := bindDataI.(*NewDatabaseBindResponse)
	if bindData.Password != password {
		t.Fatal("The password was not correct")
		return
	}
	if bindData.Username != username {
		t.Fatal("The username was not correct")
		return
	}
	if bindData.DbName != dbName {
		t.Fatal("The db name was not correct")
		return
	}
	if bindData.StardogURL != sdURL {
		t.Fatal("The url was not correct")
		return
	}

	if len(clientFactory.newUser) != 1 {
		t.Fatalf("NewUser was not called on stardog")
		return
	}
	if clientFactory.newUser[0].username != username {
		t.Fatalf("The wrong username was used")
		return
	}
	if clientFactory.newUser[0].pw != password {
		t.Fatalf("The wrong password was used")
		return
	}

	if len(clientFactory.grantUser) != 1 {
		t.Fatalf("NewUser was not called on stardog")
		return
	}
	if clientFactory.grantUser[0].username != username {
		t.Fatalf("The wrong username was used")
		return
	}
	if clientFactory.grantUser[0].dbName != dbName {
		t.Fatalf("The wrong password was used")
		return
	}

	// bind 2
	dbParams2 := newDatabaseBindParameters{}
	b2, err := json.Marshal(&dbParams2)
	if err != nil {
		t.Fatalf("failed to marshall the parameters %s", err)
		return
	}

	code, bindDataI2, err := plan.Bind(dataI, b2)
	if code != http.StatusOK {
		t.Fatalf("The status should be ok after bind %s", err)
		return
	}
	bindData2 := bindDataI2.(*NewDatabaseBindResponse)
	if bindData2.Password == "" {
		t.Fatal("The password should have been set")
		return
	}
	if bindData2.Username == "" {
		t.Fatal("The username should have been set")
		return
	}

	code, err = plan.UnBind(bindDataI)
	if err != nil {
		t.Fatal("The unbind failed")
		return
	}
	if code != http.StatusOK {
		t.Fatal("The unbind failed")
		return
	}

	if len(clientFactory.revokeUser) != 1 {
		t.Fatal("NewUser was not called on stardog")
		return
	}
	if clientFactory.revokeUser[0].username != username {
		t.Fatal("The wrong username was used")
		return
	}
	if clientFactory.revokeUser[0].dbName != dbName {
		t.Fatal("The wrong password was used")
		return
	}

	if len(clientFactory.deleteUser) != 1 {
		t.Fatalf("Delete was not called on stardog %d", len(clientFactory.deleteUser))
		return
	}
	if clientFactory.deleteUser[0].username != username {
		t.Fatal("The wrong username was used in delete")
		return
	}

	code, _, err = plan.RemoveInstance()
	if err != nil {
		t.Fatal("The remove instance command failed")
		return
	}
}
