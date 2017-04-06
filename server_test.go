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
	sqlstore "github.com/stardog-union/service-broker/store/sql"
	stardogstore "github.com/stardog-union/service-broker/store/stardog"
)

var (
	TESTURL_ENV       = "CF_TESTING_STARDOG_URL"
	MYSQL_TESTURL_ENV = "CF_TESTING_MYSQL_URL"
)

type testBrokerClient struct {
	server    *broker.Server
	brokerURL string
	username  string
	password  string
	conf      broker.ServerConfig
}

type getPlanFunc func() (*testBrokerClient, error)
type testMeat func(c *testBrokerClient) error

func getShardedDbPlanServer() (*testBrokerClient, error) {
	sdURL := os.Getenv(TESTURL_ENV)
	if sdURL == "" {
		return nil, nil
	}

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
	store, err := stardogstore.NewStardogStore(conf.BrokerID, logger, conf.Storage.Parameters)
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

func getShardedDbMySqlPlanServer() (*testBrokerClient, error) {
	sdURL := os.Getenv(TESTURL_ENV)
	if sdURL == "" {
		return nil, nil
	}

	mysqlURL := os.Getenv(MYSQL_TESTURL_ENV)
	if sdURL == "" {
		return nil, nil
	}

	b, err := ioutil.ReadFile("./testdata/shareddbmysql.json")
	if err != nil {
		return nil, err
	}
	fileStr := strings.Replace(string(b), "@@SDURL@@", sdURL, 2)
	fileStr = strings.Replace(fileStr, "@@MYSQL@@", mysqlURL, 1)

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
	store, err := sqlstore.NewMySQLStore(conf.BrokerID, logger, conf.Storage.Parameters)
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

func testDriver(t *testing.T, meatFunc testMeat, planFunc getPlanFunc) {
	c, err := planFunc()
	if err != nil {
		t.Fatalf("Error getting the server started: %s\n", err.Error())
		return
	}
	if c == nil {
		fmt.Fprintf(os.Stderr, "Skipping\n")
		return
	}
	defer c.server.Stop(true)

	err = meatFunc(c)
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func controllerGetCatalog(c *testBrokerClient) error {
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
	}
	if catalog.Services[0].Plans[0].ID == "" {
		return fmt.Errorf("there was no a plan id")
	}
	return nil
}

func TestControllerGetCatalog(t *testing.T) {
	testDriver(t, controllerGetCatalog, getShardedDbPlanServer)
}

func TestControllerGetCatalogSql(t *testing.T) {
	testDriver(t, controllerGetCatalog, getShardedDbMySqlPlanServer)
}

func controllerGetCatalogBadCreds(c *testBrokerClient) error {
	c.password = "badPw"
	bodyBuf := &bytes.Buffer{}
	_, err := doRequestResponse(c, "GET", "/v2/catalog", bodyBuf, "text/plain", 401)
	return err
}

func TestControllerGetCatalogBadCreds(t *testing.T) {
	testDriver(t, controllerGetCatalogBadCreds, getShardedDbPlanServer)
}

func TestControllerGetCatalogBadCredsSql(t *testing.T) {
	testDriver(t, controllerGetCatalogBadCreds, getShardedDbMySqlPlanServer)
}

func controllerMakeBadPlan(c *testBrokerClient) error {
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

func TestControllerMakeBadPlan(t *testing.T) {
	testDriver(t, controllerMakeBadPlan, getShardedDbPlanServer)
}

func TestControllerMakeBadPlanSql(t *testing.T) {
	testDriver(t, controllerMakeBadPlan, getShardedDbMySqlPlanServer)
}

func controllerMakeDeleteInstance(c *testBrokerClient) error {
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

func TestControllerMakeDeleteInstance(t *testing.T) {
	testDriver(t, controllerMakeDeleteInstance, getShardedDbPlanServer)
}

func TestControllerMakeDeleteInstanceSql(t *testing.T) {
	testDriver(t, controllerMakeDeleteInstance, getShardedDbMySqlPlanServer)
}

func controllerBindNoService(c *testBrokerClient) error {
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

func TestControllerBindNoService(t *testing.T) {
	testDriver(t, controllerBindNoService, getShardedDbPlanServer)
}

func TestControllerBindNoServiceSql(t *testing.T) {
	testDriver(t, controllerBindNoService, getShardedDbMySqlPlanServer)
}

func controllerDeleteBindNoService(c *testBrokerClient) error {
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

func TestControllerDeleteBindNoService(t *testing.T) {
	testDriver(t, controllerDeleteBindNoService, getShardedDbPlanServer)
}

func TestControllerDeleteBindNoServiceSql(t *testing.T) {
	testDriver(t, controllerDeleteBindNoService, getShardedDbMySqlPlanServer)
}

func controllerDeleteBindServiceExists(c *testBrokerClient) error {
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

func TestControllerDeleteBindServiceExists(t *testing.T) {
	testDriver(t, controllerDeleteBindServiceExists, getShardedDbPlanServer)
}

func TestControllerDeleteBindServiceExistsSql(t *testing.T) {
	testDriver(t, controllerDeleteBindServiceExists, getShardedDbMySqlPlanServer)
}

func controllerBindServiceExists(c *testBrokerClient) error {
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

func TestControllerBindServiceExists(t *testing.T) {
	testDriver(t, controllerBindServiceExists, getShardedDbPlanServer)
}

func TestControllerBindServiceExistsSql(t *testing.T) {
	testDriver(t, controllerBindServiceExists, getShardedDbMySqlPlanServer)
}

func controllerBindServiceTwice(c *testBrokerClient) error {
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

func TestControllerBindServiceTwice(t *testing.T) {
	testDriver(t, controllerBindServiceTwice, getShardedDbPlanServer)
}

func TestControllerBindServiceTwiceSql(t *testing.T) {
	testDriver(t, controllerBindServiceTwice, getShardedDbMySqlPlanServer)
}

func controllerMakeInstanceTwiceSame(c *testBrokerClient) error {
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

func TestControllerMakeInstanceTwiceSame(t *testing.T) {
	testDriver(t, controllerMakeInstanceTwiceSame, getShardedDbPlanServer)
}

func TestControllerMakeInstanceTwiceSameMySql(t *testing.T) {
	testDriver(t, controllerMakeInstanceTwiceSame, getShardedDbMySqlPlanServer)
}

func controllerMakeInstanceTwiceDiff(c *testBrokerClient) error {
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

func TestControllerMakeInstanceTwiceDiff(t *testing.T) {
	testDriver(t, controllerMakeInstanceTwiceDiff, getShardedDbPlanServer)
}

func TestControllerMakeInstanceTwiceDiffMySql(t *testing.T) {
	testDriver(t, controllerMakeInstanceTwiceDiff, getShardedDbMySqlPlanServer)
}

func controllerMakeInstanceTwiceDiffParams(c *testBrokerClient) error {
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

func TestControllerMakeInstanceTwiceDiffParams(t *testing.T) {
	testDriver(t, controllerMakeInstanceTwiceDiffParams, getShardedDbPlanServer)
}

func TestControllerMakeInstanceTwiceDiffParamsMySql(t *testing.T) {
	testDriver(t, controllerMakeInstanceTwiceDiffParams, getShardedDbMySqlPlanServer)
}

func controllerBindServiceTwiceSame(c *testBrokerClient) error {
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

func TestControllerBindServiceTwiceSame(t *testing.T) {
	testDriver(t, controllerBindServiceTwiceSame, getShardedDbPlanServer)
}

func TestControllerBindServiceTwiceSameySql(t *testing.T) {
	testDriver(t, controllerBindServiceTwiceSame, getShardedDbMySqlPlanServer)
}

func controllerBindServiceTwiceDiff(c *testBrokerClient) error {
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

func TestControllerBindServiceTwiceDiff(t *testing.T) {
	testDriver(t, controllerBindServiceTwiceDiff, getShardedDbPlanServer)
}

func TestControllerBindServiceTwiceDiffMySql(t *testing.T) {
	testDriver(t, controllerBindServiceTwiceDiff, getShardedDbMySqlPlanServer)
}
