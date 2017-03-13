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
	"net/http"

	"github.com/stardog-union/service-broker/broker"
)

type dataBasePlanFactory struct {
	StardogURL string `json:"stardog_url"`
	AdminName  string `json:"admin_username"`
	AdminPw    string `json:"admin_password"`
	planIDStr  string
}

type serviceParameters struct {
	DbName string `json:"db_name"`
}

type newDatabasePlan struct {
	url           string
	adminName     string
	adminPw       string
	params        newDatabasePlanParameters
	planID        string
	clientFactory broker.StardogClientFactory
	logger        broker.SdLogger
}

type newDatabasePlanParameters struct {
	DbName string `json:"db_name"`
}

// NewDatabaseBindResponse is the response document that is returned from the Bind call
type NewDatabaseBindResponse struct {
	DbName     string `json:"db_name"`
	StardogURL string `json:"url"`
	Password   string `json:"password"`
	Username   string `json:"username"`
}

type newDatabaseBindParameters struct {
	Password string `json:"password, omitempty"`
	Username string `json:"username, omitempty"`
}

// GetPlanFactory returns a PlanFactory for the shared database plan
func GetPlanFactory(planID string, params interface{}) (broker.PlanFactory, error) {
	var dbPlan dataBasePlanFactory

	err := broker.ReSerializeInterface(params, &dbPlan)
	if err != nil {
		return nil, err
	}
	dbPlan.planIDStr = planID
	return &dbPlan, nil
}

func (df *dataBasePlanFactory) MakePlan(clientFactory broker.StardogClientFactory, logger broker.SdLogger) broker.Plan {
	return &newDatabasePlan{
		url:           df.StardogURL,
		adminName:     df.AdminName,
		adminPw:       df.AdminPw,
		planID:        df.PlanID(),
		clientFactory: clientFactory,
		logger:        logger,
	}
}

func (df *dataBasePlanFactory) InflatePlan(serviceInstance *broker.ServiceInstance, clientFactory broker.StardogClientFactory, logger broker.SdLogger) (broker.Plan, error) {
	serviceParams, err := reInflateService(serviceInstance.InstanceParams)
	if err != nil {
		return nil, err
	}
	serviceInstance.PlanID = df.PlanID()
	p := &newDatabasePlan{
		url:           df.StardogURL,
		adminName:     df.AdminName,
		adminPw:       df.AdminPw,
		planID:        df.PlanID(),
		clientFactory: clientFactory,
		logger:        logger,
		params:        newDatabasePlanParameters{DbName: serviceParams.DbName},
	}
	return p, nil
}

func (df *dataBasePlanFactory) PlanName() string {
	return "shareddb"
}

func (df *dataBasePlanFactory) PlanDescription() string {
	return "Creates a new Stardog database on an existing server.  The " +
		"Stardog server maybe shared by many applications."
}

func (df *dataBasePlanFactory) PlanID() string {
	return df.planIDStr
}

func (df *dataBasePlanFactory) Metadata() interface{} {
	return nil
}

func (df *dataBasePlanFactory) Free() bool {
	return true
}

func (df *dataBasePlanFactory) Bindable() bool {
	return true
}

func (p *newDatabasePlan) CreateServiceInstance(parameters []byte) (int, interface{}, error) {
	err := json.Unmarshal(parameters, &p.params)
	if err != nil {
		return http.StatusBadRequest, nil, fmt.Errorf("The parameters were not properly formed")
	}

	if p.params.DbName == "" {
		p.params.DbName = broker.GetRandomName("db", 16)
	}
	outParams := serviceParameters{DbName: p.params.DbName}

	client := p.clientFactory.GetStardogAdminClient(
		p.url,
		broker.DatabaseCredentials{
			Username: p.adminName,
			Password: p.adminPw})

	// Create an instance database for storing bindings
	err = client.CreateDatabase(outParams.DbName)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusCreated, outParams, nil
}

func (p *newDatabasePlan) RemoveInstance() (int, interface{}, error) {
	// Delete the db if it was created
	client := p.clientFactory.GetStardogAdminClient(
		p.url,
		broker.DatabaseCredentials{
			Username: p.adminName,
			Password: p.adminPw})
	err := client.DeleteDatabase(p.params.DbName)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, &broker.CreateGetServiceInstanceResponse{}, nil
}

func reInflateService(service interface{}) (*serviceParameters, error) {
	b, err := json.Marshal(service)
	if err != nil {
		return nil, err
	}
	var sp serviceParameters
	err = json.Unmarshal(b, &sp)
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

func (p *newDatabasePlan) Bind(service interface{}, parameters []byte) (int, interface{}, error) {
	var params newDatabaseBindParameters

	err := json.Unmarshal(parameters, &params)
	if err != nil {
		return http.StatusBadRequest, nil, fmt.Errorf("The parameters were not properly formed")
	}

	if params.Username == "" {
		params.Username = broker.GetRandomName("stardog", 8)
	}
	if params.Password == "" {
		params.Password = broker.GetRandomName("", 24)
	}
	serviceParams, err := reInflateService(service)
	if err != nil {
		return http.StatusBadRequest, nil, fmt.Errorf("The plan specific parameters were poorly formed")
	}

	client := p.clientFactory.GetStardogAdminClient(
		p.url,
		broker.DatabaseCredentials{
			Username: p.adminName,
			Password: p.adminPw})
	responseCred := NewDatabaseBindResponse{
		Username:   params.Username,
		Password:   params.Password,
		DbName:     serviceParams.DbName,
		StardogURL: p.url,
	}

	e, err := client.UserExists(responseCred.Username)
	if err != nil {
		p.logger.Logf(broker.WARN, "UserExists check failed: %s", err)
		return http.StatusInternalServerError, nil, fmt.Errorf("UserExists check failed")
	}
	if e {
		return http.StatusConflict, nil, fmt.Errorf("Failed to create the user because %s already exists", responseCred.Username)
	}
	err = client.NewUser(responseCred.Username, responseCred.Password)
	if err != nil {
		p.logger.Logf(broker.WARN, "Failed to create the user %s", err)
		return http.StatusInternalServerError, nil, fmt.Errorf("Failed to create the user")
	}
	err = client.GrantUserAccessToDb(responseCred.DbName, responseCred.Username)
	if err != nil {
		p.logger.Logf(broker.INFO, "Failed to grant access on %s to the user %s: %s", responseCred.DbName, responseCred.Username, err)
		return http.StatusInternalServerError, nil, fmt.Errorf("Failed to grant access on %s to the user %s", responseCred.DbName, responseCred.Username)
	}
	return http.StatusOK, &responseCred, nil
}

func bindingInflate(binding interface{}) (*NewDatabaseBindResponse, error) {
	b, err := json.Marshal(binding)
	if err != nil {
		return nil, err
	}
	var bp NewDatabaseBindResponse
	err = json.Unmarshal(b, &bp)
	if err != nil {
		return nil, err
	}
	return &bp, nil
}

func (p *newDatabasePlan) UnBind(binding interface{}) (int, error) {
	client := p.clientFactory.GetStardogAdminClient(p.url,
		broker.DatabaseCredentials{
			Username: p.adminName,
			Password: p.adminPw})
	serviceBinding, err := bindingInflate(binding)
	if err != nil {
		p.logger.Logf(broker.WARN, "Failed to inflate the parameters %s", err)
		return http.StatusInternalServerError, err
	}
	err = client.RevokeUserAccess(serviceBinding.DbName, serviceBinding.Username)
	if err != nil {
		p.logger.Logf(broker.WARN, "Failed to revoke user accesss %s", err)
		return http.StatusInternalServerError, err
	}

	err = client.DeleteUser(serviceBinding.Username)
	if err != nil {
		p.logger.Logf(broker.WARN, "Failed to delete user %s: %s", serviceBinding.Username, err)
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func (p *newDatabasePlan) PlanID() string {
	return p.planID
}

func (p *newDatabasePlan) EqualInstance(requestParamsI interface{}) bool {
	var requestParams newDatabasePlanParameters

	err := broker.ReSerializeInterface(requestParamsI, &requestParams)
	if err != nil {
		return false
	}

	return requestParams.DbName == p.params.DbName
}

func (p *newDatabasePlan) EqualBinding(bindInstance *broker.BindInstance, bindRequest *broker.BindRequest) bool {
	var bindParams newDatabaseBindParameters

	err := broker.ReSerializeInterface(bindRequest.Parameters, &bindParams)
	if err != nil {
		return false
	}
	var bindInstanceParams NewDatabaseBindResponse
	err = broker.ReSerializeInterface(bindInstance.PlanParams, &bindInstanceParams)
	if err != nil {
		return false
	}

	return bindParams.Password == bindInstanceParams.Password && bindParams.Username == bindInstanceParams.Username
}
