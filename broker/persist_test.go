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

package broker

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"
)

var (
	TESTURL_ENV string = "CF_TESTING_STARDOG_URL"
)

type someData struct {
	Word string `json:"word"`
}

func TestStardogPersistence(t *testing.T) {
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	logger, _ := NewSdLogger(baseLogger, "DEBUG")

	sdURL := os.Getenv(TESTURL_ENV)
	if sdURL == "" {
		t.Skipf("The env %s must be set to run this test", TESTURL_ENV)
	}

	params := stardogMetadataStore{
		AdminName:  "admin",
		AdminPw:    "admin",
		StardogURL: sdURL,
	}
	store, err := NewStardogStore(
		"stardog-service-0A48E1D9-DCC9-4A61-8677-CB10B3C05512",
		logger,
		&params)
	if err != nil {
		fmt.Printf("ERROR %s\n", err)
		t.Fatal()
	}

	w := someData{Word: "instanceword"}
	instanceGUID := fmt.Sprintf("SERVICEINSTANCE%d", rand.Int63())
	si := ServiceInstance{
		InstanceGUID:   instanceGUID,
		PlanID:         "PlanID",
		InstanceParams: w,
	}

	err = store.AddInstance(instanceGUID, &si)
	if err != nil {
		fmt.Printf("ERROR %s\n", err)
		t.Fatal()
	}

	_, err = store.GetInstance(instanceGUID)
	if err != nil {
		fmt.Printf("ERROR %s\n", err)
		t.Fatal()
	}

	w = someData{Word: "bind_word_0"}
	bindGUID := fmt.Sprintf("Binding1-%d", rand.Int63())
	bi := BindInstance{
		BindGUID:   bindGUID,
		PlanParams: &w,
	}
	store.AddBinding(instanceGUID, bindGUID, &bi)
	w = someData{Word: "bind1word"}
	bindGUID = fmt.Sprintf("Binding2-%d", rand.Int63())
	bi = BindInstance{
		BindGUID:   bindGUID,
		PlanParams: &w,
	}
	store.AddBinding(instanceGUID, bindGUID, &bi)
	w = someData{Word: "bindword2"}
	bindGUID = fmt.Sprintf("Binding3-%d", rand.Int63())
	bi = BindInstance{
		BindGUID:   bindGUID,
		PlanParams: &w,
	}
	store.AddBinding(instanceGUID, bindGUID, &bi)

	bis, err := store.GetAllBindings(instanceGUID)
	if err != nil {
		fmt.Printf("ERROR %s\n", err)
		t.Fatal()
	}
	if len(bis) != 3 {
		fmt.Printf("ERROR not enough bindings")
		t.Fatal()
	}

	for _, v := range bis {
		bi, err := store.GetBinding(instanceGUID, v.BindGUID)
		if err != nil {
			fmt.Printf("ERROR %s\n", err)
			t.Fatal()
		}
		if bi.BindGUID != v.BindGUID {
			fmt.Printf("GUID not the same\n")
			t.Fatal()
		}
		err = store.DeleteBinding(instanceGUID, bi.BindGUID)
		if err != nil {
			fmt.Printf("ERROR %s\n", err)
			t.Fatal()
		}
		_, err = store.GetBinding(instanceGUID, v.BindGUID)
		if err == nil {
			fmt.Printf("The binding should be gone now %s\n", err)
			t.Fatal()
		}
	}

	err = store.DeleteInstance(instanceGUID)
	if err != nil {
		fmt.Printf("ERROR %s\n", err)
		t.Fatal()
	}
	siBack2, err := store.GetInstance(instanceGUID)
	if err == nil {
		fmt.Printf("The instance should be code %s\n", siBack2)
		t.Fatal()
	}
}
