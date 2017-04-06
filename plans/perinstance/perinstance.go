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

type perInstancePlanFactory struct {
	planIDStr string
	logger    broker.SdLogger
}

type createServiceParameters struct {
	DbName     string `json:"db_name"`
	StardogURL string `json:"url"`
	Password   string `json:"password"`
	Username   string `json:"username"`
}

type perInstanceDatabasePlan struct {
	planID        string
	clientFactory broker.StardogClientFactory
	logger        broker.SdLogger
	param         createServiceParameters
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
	var dbPlan perInstancePlanFactory

	// It is currently empty but leaving as a placeholder
	err := broker.ReSerializeInterface(params, &dbPlan)
	if err != nil {
		return nil, err
	}
	dbPlan.planIDStr = planID
	return &dbPlan, nil
}

func (df *perInstancePlanFactory) InflatePlan(instanceParams interface{}, clientFactory broker.StardogClientFactory, logger broker.SdLogger) (broker.Plan, error) {
	var serviceParams createServiceParameters
	err := broker.ReSerializeInterface(instanceParams, &serviceParams)
	if err != nil {
		return nil, err
	}

	if serviceParams.StardogURL == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("A Stardog URL is required")
	}
	if serviceParams.Password == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("An admin password is required")
	}
	if serviceParams.Username == "" {
		serviceParams.Username = "admin"
	}
	if serviceParams.DbName == "" {
		serviceParams.DbName = broker.GetRandomName("db", 16)
	}
	p := &perInstanceDatabasePlan{
		planID:        df.PlanID(),
		clientFactory: clientFactory,
		logger:        logger,
		param:         serviceParams,
	}
	return p, nil
}

func (df *perInstancePlanFactory) PlanName() string {
	return "perinstance"
}

func (df *perInstancePlanFactory) PlanDescription() string {
	return "Associate each instance with an existing Stardog knowledge graph."
}

func (df *perInstancePlanFactory) PlanID() string {
	return df.planIDStr
}

func (df *perInstancePlanFactory) Metadata() interface{} {
	return nil
}

func (df *perInstancePlanFactory) Free() bool {
	return true
}

func (df *perInstancePlanFactory) Bindable() bool {
	return true
}

func (p *perInstanceDatabasePlan) CreateServiceInstance() (int, interface{}, error) {
	client := p.clientFactory.GetStardogAdminClient(
		p.params.StardogURL,
		broker.DatabaseCredentials{
			Username: p.params.Username,
			Password: p.params.Password})

	// Create an instance database for storing bindings
	err = client.CreateDatabase(p.params.DbName)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusCreated, p.params, nil
}

func (p *perInstanceDatabasePlan) RemoveInstance() (int, interface{}, error) {
	// Delete the db if it was created
	client := p.clientFactory.GetStardogAdminClient(
		p.StardogURL,
		broker.DatabaseCredentials{
			Username: p.,
			Password: p.adminPw})
	err := client.DeleteDatabase(p.params.DbName)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, &broker.CreateGetServiceInstanceResponse{}, nil
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
