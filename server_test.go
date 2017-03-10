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
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stardog-union/service-broker/broker"
	"github.com/stardog-union/service-broker/plans/shared"
)

var (
	TESTURL_ENV = "CF_TESTING_STARDOG_URL"
)

type testBrokerClient struct {
	server    *broker.Server
	brokerURL string
	username  string
	password  string
	conf      broker.ServerConfig
}

func getShardedDbPlanServer(sdURL string) (*testBrokerClient, error) {
	b, err := ioutil.ReadFile("./testdata/shareddb.json")
	if err != nil {
		return nil, err
	}
	fileStr := strings.Replace(string(b), "@@SDURL@@", sdURL, 2)

	var conf broker.ServerConfig
	err = json.Unmarshal([]byte(fileStr), &conf)
	if err != nil {
		return nil, err
	}
	dbPlanMap, err := handlePlugins(&conf)
	if err != nil {
		return nil, err
	}
	baseLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime)
	logger, err := broker.NewSdLogger(baseLogger, conf.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed create the logger %s\n", err)
		return nil, err
	}
	store, err := broker.NewStardogStore(conf.BrokerID, logger, conf.Storage.Parameters)
	if err != nil {
		return nil, fmt.Errorf("Error setting up the data store: %s", err)
	}

	clientFactory := broker.NewClientFactory(logger)
	s, err := broker.CreateServer(dbPlanMap, &conf, clientFactory, logger, store)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Starting the server\n")
	err = s.Start()
	if err != nil {
		return nil, err
	}
	fmt.Printf("Getting the addr\n")
	addr, err := s.GetAddr()
	if err != nil {
		s.Stop(true)
		return nil, err
	}
	addrS := addr.String()
	ndx := strings.LastIndex(addrS, ":")
	url := "http://localhost" + addrS[ndx:]

	c := testBrokerClient{
		brokerURL: url,
		server:    s,
		username:  conf.BrokerUsername,
		password:  conf.BrokerPassword,
		conf:      conf,
	}

	return &c, nil
}

func doRequestResponse(c *testBrokerClient, method, path string, body io.Reader, contentType string, expectedCode int) ([]byte, error) {
	urlStr := fmt.Sprintf("%s%s", c.brokerURL, path)
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	client := &http.Client{}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed do the post %s", err)
	}
	if resp.StatusCode != expectedCode {
		resp.Body.Close()
		return nil, fmt.Errorf("Expected %d but got %d when %s to %s", expectedCode, resp.StatusCode, method, urlStr)
	}
	content, err := ioutil.ReadAll(resp.Body)

	return content, nil
}

func testUUID() (uuid string) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	return
}

type testMeat func(c *testBrokerClient) error

func testDriver(t *testing.T, meatFunc testMeat) {
	sdURL := os.Getenv(TESTURL_ENV)
	if sdURL == "" {
		t.Skipf("The env %s must be set to run this test", TESTURL_ENV)
		return
	}
	c, err := getShardedDbPlanServer(sdURL)
	if err != nil {
		t.Fatalf("Error getting the server started: %s\n", err.Error())
		return
	}
	defer c.server.Stop(true)

	err = meatFunc(c)
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestControllerGetCatalog(t *testing.T) {
	f := func(c *testBrokerClient) error {
		bodyBuf := &bytes.Buffer{}
		data, err := doRequestResponse(c, "GET", "/v2/catalog", bodyBuf, "text/plain", 200)
		if err != nil {
			fmt.Printf("ERROR %s\n", err)
			return err
		}
		var catalog broker.CatalogResponse
		err = json.Unmarshal(data, &catalog)
		if err != nil {
			return err
		}
		if len(catalog.Services) != 1 {
			return fmt.Errorf("the catalog services were empty")
		}
		if !catalog.Services[0].Bindable {
			return fmt.Errorf("the catalog service was not empty")
		}
		if catalog.Services[0].Name != "Stardog" {
			return fmt.Errorf("the catalog service was not empty")
		}
		if len(catalog.Services[0].Plans) != 1 {
			return fmt.Errorf("there was not a plan")
		}if len(catalog.Services[0].Plans[0].ID) != "" {
			return fmt.Errorf("there was no a plan id")
		}
		fmt.Printf("CATALOG %s\n", string(data))
		return nil
	}
	testDriver(t, f)
}

func TestControllerGetCatalogBadCreds(t *testing.T) {
	f := func(c *testBrokerClient) error {
		c.password = "badPw"
		bodyBuf := &bytes.Buffer{}
		_, err := doRequestResponse(c, "GET", "/v2/catalog", bodyBuf, "text/plain", 401)
		return err
	}

	testDriver(t, f)
}

func TestControllerMakeBadPlan(t *testing.T) {
	f := func(c *testBrokerClient) error {
		path := fmt.Sprintf("/v2/service_instances/%s", testUUID())

		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           testUUID(),
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 400)
		if err != nil {
			return err
		}
		_, err = doRequestResponse(c, "GET", path, bodyBuf, "application/json", 404)
		return err
	}

	testDriver(t, f)
}

func TestControllerMakeDeleteInstance(t *testing.T) {
	f := func(c *testBrokerClient) error {
		path := fmt.Sprintf("/v2/service_instances/%s", testUUID())

		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}
		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "GET", path, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}
		byteBuf = &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", path, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		_, err = doRequestResponse(c, "GET", path, bodyBuf, "application/json", 404)
		if err != nil {
			return err
		}
		return nil
	}

	testDriver(t, f)
}

func TestControllerBindNoService(t *testing.T) {
	f := func(c *testBrokerClient) error {
		serviceID := testUUID()
		bindID := testUUID()
		path := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID)

		bindReq := broker.BindRequest{
			ServiceID: c.conf.BrokerID,
			PlanID:    c.conf.Plans[0].PlanID,
		}

		data, err := json.Marshal(bindReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 500)
		if err != nil {
			return err
		}
		return nil
	}

	testDriver(t, f)
}

func TestControllerDeleteBindNoService(t *testing.T) {
	f := func(c *testBrokerClient) error {
		serviceID := testUUID()
		bindID := testUUID()
		path := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID)

		bindReq := broker.BindRequest{
			ServiceID: c.conf.BrokerID,
			PlanID:    c.conf.Plans[0].PlanID,
		}

		data, err := json.Marshal(bindReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "DELETE", path, bodyBuf, "application/json", 410)
		if err != nil {
			return err
		}
		return nil
	}

	testDriver(t, f)
}

func TestControllerDeleteBindServiceExists(t *testing.T) {
	f := func(c *testBrokerClient) error {
		serviceID := testUUID()

		servicePath := fmt.Sprintf("/v2/service_instances/%s", serviceID)

		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", servicePath, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		bindID := testUUID()
		path := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID)

		bindReq := broker.BindRequest{
			ServiceID: c.conf.BrokerID,
			PlanID:    c.conf.Plans[0].PlanID,
		}

		data, err = json.Marshal(bindReq)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data))

		_, err = doRequestResponse(c, "DELETE", path, bodyBuf, "application/json", 410)
		if err != nil {
			return err
		}

		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", servicePath, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		return nil
	}

	testDriver(t, f)
}

func TestControllerBindServiceExists(t *testing.T) {
	f := func(c *testBrokerClient) error {
		serviceID := testUUID()

		servicePath := fmt.Sprintf("/v2/service_instances/%s", serviceID)

		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", servicePath, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		bindID := testUUID()
		path := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID)

		bindReq := broker.BindRequest{
			ServiceID: c.conf.BrokerID,
			PlanID:    c.conf.Plans[0].PlanID,
		}

		data, err = json.Marshal(bindReq)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data))

		resp, err := doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}
		var bindR broker.BindResponse
		err = json.Unmarshal(resp, &bindR)
		if err != nil {
			return err
		}
		var creds shared.NewDatabaseBindResponse
		err = broker.ReSerializeInterface(&bindR.Credentials, &creds)
		if err != nil {
			return err
		}

		persistConfMap := c.conf.Plans[0].Parameters.(map[string]interface{})
		if creds.StardogURL != persistConfMap["stardog_url"].(string) {
			return fmt.Errorf("The credentials have the wrong the URL")
		}
		if creds.DbName == "" || creds.Username == "" || creds.Password == "" {
			return fmt.Errorf("The credentials were not properly set")
		}

		_, err = doRequestResponse(c, "DELETE", path, bodyBuf, "application/json", 200)
		if err != nil {
			return err
		}

		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", servicePath, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		return nil
	}

	testDriver(t, f)
}

func TestControllerBindServiceTwice(t *testing.T) {
	f := func(c *testBrokerClient) error {
		serviceID := testUUID()

		servicePath := fmt.Sprintf("/v2/service_instances/%s", serviceID)

		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", servicePath, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		bindID := testUUID()
		path := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID)

		bindReq := broker.BindRequest{
			ServiceID: c.conf.BrokerID,
			PlanID:    c.conf.Plans[0].PlanID,
		}

		data, err = json.Marshal(bindReq)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data))

		resp, err := doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}
		var bindR broker.BindResponse
		err = json.Unmarshal(resp, &bindR)
		if err != nil {
			return err
		}
		var creds shared.NewDatabaseBindResponse
		err = broker.ReSerializeInterface(&bindR.Credentials, &creds)
		if err != nil {
			return err
		}

		params := make(map[string]string)
		params["username"] = broker.GetRandomName("newName", 6)
		params["password"] = creds.Password

		bindReq2 := broker.BindRequest{
			ServiceID:  c.conf.BrokerID,
			PlanID:     c.conf.Plans[0].PlanID,
			Parameters: params,
		}

		data2, err := json.Marshal(bindReq2)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data2))

		bindID2 := testUUID()
		path2 := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID2)

		resp2, err := doRequestResponse(c, "PUT", path2, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}
		var bindR2 broker.BindResponse
		err = json.Unmarshal(resp2, &bindR2)
		if err != nil {
			return err
		}
		var creds2 shared.NewDatabaseBindResponse
		err = broker.ReSerializeInterface(&bindR2.Credentials, &creds2)
		if err != nil {
			return err
		}

		_, err = doRequestResponse(c, "DELETE", path, bodyBuf, "application/json", 200)
		if err != nil {
			return err
		}

		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", servicePath, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		return nil
	}

	testDriver(t, f)
}

func TestControllerMakeInstanceTwiceSame(t *testing.T) {
	f := func(c *testBrokerClient) error {
		path := fmt.Sprintf("/v2/service_instances/%s", testUUID())

		iParams := make(map[string]interface{})
		iParams["db_name"] = broker.GetRandomName("aname", 4)
		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
			Parameters:       iParams,
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 200)
		if err != nil {
			return err
		}
		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", path, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		_, err = doRequestResponse(c, "GET", path, bodyBuf, "application/json", 404)
		if err != nil {
			return err
		}
		return nil
	}

	testDriver(t, f)
}

func TestControllerMakeInstanceTwiceDiff(t *testing.T) {
	f := func(c *testBrokerClient) error {
		path := fmt.Sprintf("/v2/service_instances/%s", testUUID())

		iParams := make(map[string]interface{})
		iParams["db_name"] = broker.GetRandomName("aname", 4)
		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
			Parameters:       iParams,
		}
		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		serviceInstanceReq2 := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
			Parameters:       iParams,
		}
		data2, err := json.Marshal(serviceInstanceReq2)
		if err != nil {
			return err
		}
		bodyBuf2 := strings.NewReader(string(data2))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf2, "application/json", 409)
		if err != nil {
			return err
		}
		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", path, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		_, err = doRequestResponse(c, "GET", path, bodyBuf, "application/json", 404)
		if err != nil {
			return err
		}
		return nil
	}

	testDriver(t, f)
}

func TestControllerMakeInstanceTwiceDiffParams(t *testing.T) {
	f := func(c *testBrokerClient) error {
		path := fmt.Sprintf("/v2/service_instances/%s", testUUID())

		iParams := make(map[string]interface{})
		iParams["db_name"] = broker.GetRandomName("aname", 4)
		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
			Parameters:       iParams,
		}
		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		iParams2 := make(map[string]interface{})
		iParams2["db_name"] = broker.GetRandomName("bname", 4)
		serviceInstanceReq.Parameters = iParams2
		data2, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf2 := strings.NewReader(string(data2))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf2, "application/json", 409)
		if err != nil {
			return err
		}
		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", path, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		_, err = doRequestResponse(c, "GET", path, bodyBuf, "application/json", 404)
		if err != nil {
			return err
		}
		return nil
	}

	testDriver(t, f)
}

func TestControllerBindServiceTwiceSame(t *testing.T) {
	f := func(c *testBrokerClient) error {
		serviceID := testUUID()

		servicePath := fmt.Sprintf("/v2/service_instances/%s", serviceID)

		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", servicePath, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		bindID := testUUID()
		path := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID)

		params := make(map[string]string)
		params["username"] = broker.GetRandomName("newName", 6)
		params["password"] = broker.GetRandomName("pw", 6)

		bindReq := broker.BindRequest{
			ServiceID:  c.conf.BrokerID,
			PlanID:     c.conf.Plans[0].PlanID,
			Parameters: params,
		}

		data, err = json.Marshal(bindReq)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		bodyBuf = strings.NewReader(string(data))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 200)
		if err != nil {
			return err
		}

		_, err = doRequestResponse(c, "DELETE", path, bodyBuf, "application/json", 200)
		if err != nil {
			return err
		}

		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", servicePath, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		return nil
	}

	testDriver(t, f)
}

func TestControllerBindServiceTwiceDiff(t *testing.T) {
	f := func(c *testBrokerClient) error {
		serviceID := testUUID()

		servicePath := fmt.Sprintf("/v2/service_instances/%s", serviceID)

		serviceInstanceReq := broker.CreateServiceInstanceRequest{
			ServiceID:        c.conf.BrokerID,
			PlanID:           c.conf.Plans[0].PlanID,
			OrganizationGUID: testUUID(),
			SpaceGUID:        testUUID(),
		}

		data, err := json.Marshal(serviceInstanceReq)
		if err != nil {
			return err
		}
		bodyBuf := strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", servicePath, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}

		bindID := testUUID()
		path := fmt.Sprintf("/v2/service_instances/%s/service_bindings/%s", serviceID, bindID)

		bindReq := broker.BindRequest{
			ServiceID: c.conf.BrokerID,
			PlanID:    c.conf.Plans[0].PlanID,
		}

		data, err = json.Marshal(bindReq)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data))

		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 201)
		if err != nil {
			return err
		}
		params := make(map[string]string)
		params["username"] = broker.GetRandomName("newName", 6)

		bindReq2 := broker.BindRequest{
			ServiceID:  c.conf.BrokerID,
			PlanID:     c.conf.Plans[0].PlanID,
			Parameters: params,
		}
		data2, err := json.Marshal(bindReq2)
		if err != nil {
			return err
		}
		bodyBuf = strings.NewReader(string(data2))
		_, err = doRequestResponse(c, "PUT", path, bodyBuf, "application/json", 409)
		if err != nil {
			return err
		}

		_, err = doRequestResponse(c, "DELETE", path, bodyBuf, "application/json", 200)
		if err != nil {
			return err
		}

		byteBuf := &bytes.Buffer{}
		_, err = doRequestResponse(c, "DELETE", servicePath, byteBuf, "application/json", 200)
		if err != nil {
			return err
		}

		return nil
	}

	testDriver(t, f)
}
