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

package store

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"

	"github.com/stardog-union/service-broker/broker"
	storesql "github.com/stardog-union/service-broker/store/sql"
	storestardog "github.com/stardog-union/service-broker/store/stardog"
)

var (
	TESTURL_ENV       = "CF_TESTING_STARDOG_URL"
	MYSQL_TESTURL_ENV = "CF_TESTING_MYSQL_URL"
)

type someData struct {
	Word string `json:"word"`
}

func simpleWalkthrough(store broker.Store) error {
	var err error
	w := someData{Word: "instanceword"}
	instanceGUID := fmt.Sprintf("SERVICEINSTANCE%d", rand.Int63())
	si := broker.ServiceInstance{
		InstanceGUID:   instanceGUID,
		PlanID:         "PlanID",
		InstanceParams: w,
	}

	fmt.Printf("Pre AddInstance\n")
	err = store.AddInstance(instanceGUID, &si)
	if err != nil {
		return err
	}
	fmt.Printf("Pre GetInstance\n")
	_, err = store.GetInstance(instanceGUID)
	if err != nil {
		return err
	}

	w = someData{Word: "bind_word_0"}
	bindGUID := fmt.Sprintf("Binding1-%d", rand.Int63())
	bi := broker.BindInstance{
		BindGUID:   bindGUID,
		PlanParams: &w,
	}
	fmt.Printf("Pre AddBinding\n")
	store.AddBinding(instanceGUID, bindGUID, &bi)
	w = someData{Word: "bind1word"}
	bindGUID = fmt.Sprintf("Binding2-%d", rand.Int63())
	bi = broker.BindInstance{
		BindGUID:   bindGUID,
		PlanParams: &w,
	}
	fmt.Printf("Pre AddBinding 2\n")
	store.AddBinding(instanceGUID, bindGUID, &bi)
	w = someData{Word: "bindword2"}
	bindGUID = fmt.Sprintf("Binding3-%d", rand.Int63())
	bi = broker.BindInstance{
		BindGUID:   bindGUID,
		PlanParams: &w,
	}
	fmt.Printf("Pre AddBinding 3\n")
	store.AddBinding(instanceGUID, bindGUID, &bi)

	fmt.Printf("Pre GetAllBindings\n")
	bis, err := store.GetAllBindings(instanceGUID)
	if err != nil {
		return err
	}
	if len(bis) != 3 {
		return fmt.Errorf("ERROR not enough bindings %d", len(bis))
	}

	for _, v := range bis {
		fmt.Printf("Pre GetBinding\n")
		bi, err := store.GetBinding(instanceGUID, v.BindGUID)
		if err != nil {
			return err
		}
		if bi.BindGUID != v.BindGUID {
			return fmt.Errorf("GUID not the same %s != %s", bi.BindGUID, v.BindGUID)
		}
		fmt.Printf("Pre DeleteBinding\n")
		err = store.DeleteBinding(instanceGUID, bi.BindGUID)
		if err != nil {
			return err
		}
		fmt.Printf("Pre GetBinding 2\n")
		_, err = store.GetBinding(instanceGUID, v.BindGUID)
		if err == nil {
			return err
		}
	}

	fmt.Printf("Pre DeleteInstance\n")
	err = store.DeleteInstance(instanceGUID)
	if err != nil {
		return err
	}
	fmt.Printf("Pre GetInstance\n")
	siBack2, err := store.GetInstance(instanceGUID)
	if err == nil {
		return fmt.Errorf("%s %s", siBack2, err)
	}
	return nil
}

func TestStardogPersistence(t *testing.T) {
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	logger, _ := broker.NewSdLogger(baseLogger, "DEBUG")

	sdURL := os.Getenv(TESTURL_ENV)
	if sdURL == "" {
		t.Skipf("The env %s must be set to run this test", TESTURL_ENV)
	}
	paramsMap := make(map[string]string)
	paramsMap["admin_username"] = "admin"
	paramsMap["admin_password"] = "admin"
	paramsMap["stardog_url"] = sdURL
	store, err := storestardog.NewStardogStore(
		"stardog-service-0A48E1D9-DCC9-4A61-8677-CB10B3C05512",
		logger,
		&paramsMap)
	if err != nil {
		fmt.Printf("ERROR %s\n", err)
		t.Fatal()
	}
	err = simpleWalkthrough(store)
	if err != nil {
		t.Fatalf("%s", err)
	}
}

func TestSQLPersistence(t *testing.T) {
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	logger, _ := broker.NewSdLogger(baseLogger, "DEBUG")

	mySQLUrl := os.Getenv(MYSQL_TESTURL_ENV)
	if mySQLUrl == "" {
		t.Skipf("The env %s must be set to run this test", MYSQL_TESTURL_ENV)
	}

	paramsMap := make(map[string]interface{})
	paramsMap["use_cap"] = false
	paramsMap["sql_driver_name"] = "mysql"
	paramsMap["contact_string"] = mySQLUrl
	paramsMap["database_name"] = broker.GetRandomName("metadb", 8)
	store, err := storesql.NewMySQLStore(
		"stardog-service-0A48E1D9-DCC9-4A61-8677-CB10B3C05512",
		logger,
		&paramsMap)
	if err != nil {
		fmt.Printf("ERROR %s\n", err)
		t.Fatal()
	}
	err = simpleWalkthrough(store)
	if err != nil {
		t.Fatalf("%s", err)
	}
}
