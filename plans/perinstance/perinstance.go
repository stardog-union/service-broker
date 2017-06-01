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

package perinstance

import (
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
type BindResponse struct {
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
		return nil, fmt.Errorf("A Stardog URL is required")
	}
	if serviceParams.Password == "" {
		return nil, fmt.Errorf("An admin password is required")
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
	return "Associate each instance with an existing Stardog Knowledge Graph."
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
		p.param.StardogURL,
		broker.DatabaseCredentials{
			Username: p.param.Username,
			Password: p.param.Password})

	// Create an instance database for storing bindings
	err := client.CreateDatabase(p.param.DbName)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusCreated, p.param, nil
}

func (p *perInstanceDatabasePlan) RemoveInstance() (int, interface{}, error) {
	// Delete the db if it was created
	client := p.clientFactory.GetStardogAdminClient(
		p.param.StardogURL,
		broker.DatabaseCredentials{
			Username: p.param.Username,
			Password: p.param.Password})
	err := client.DeleteDatabase(p.param.DbName)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, &broker.CreateGetServiceInstanceResponse{}, nil
}

func (p *perInstanceDatabasePlan) Bind(parameters interface{}) (int, interface{}, error) {
	var params newDatabaseBindParameters

	err := broker.ReSerializeInterface(parameters, &params)
	if err != nil {
		return http.StatusBadRequest, nil, fmt.Errorf("The parameters were not properly formed")
	}

	if params.Username == "" {
		params.Username = broker.GetRandomName("stardog", 8)
	}
	if params.Password == "" {
		params.Password = broker.GetRandomName("", 24)
	}

	client := p.clientFactory.GetStardogAdminClient(
		p.param.StardogURL,
		broker.DatabaseCredentials{
			Username: p.param.Username,
			Password: p.param.Password})
	responseCred := BindResponse{
		Username:   params.Username,
		Password:   params.Password,
		DbName:     p.param.DbName,
		StardogURL: p.param.StardogURL,
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

func (p *perInstanceDatabasePlan) UnBind(binding interface{}) (int, error) {
	var bindResponse BindResponse
	client := p.clientFactory.GetStardogAdminClient(p.param.StardogURL,
		broker.DatabaseCredentials{
			Username: p.param.Username,
			Password: p.param.Password})

	err := broker.ReSerializeInterface(binding, &bindResponse)
	if err != nil {
		p.logger.Logf(broker.WARN, "Failed to inflate the parameters %s", err)
		return http.StatusInternalServerError, err
	}
	err = client.RevokeUserAccess(bindResponse.DbName, bindResponse.Username)
	if err != nil {
		p.logger.Logf(broker.WARN, "Failed to revoke user accesss %s", err)
		return http.StatusInternalServerError, err
	}

	err = client.DeleteUser(bindResponse.Username)
	if err != nil {
		p.logger.Logf(broker.WARN, "Failed to delete user %s: %s", bindResponse.Username, err)
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func (p *perInstanceDatabasePlan) PlanID() string {
	return p.planID
}

func (p *perInstanceDatabasePlan) EqualInstance(requestParamsI interface{}) bool {
	var bindResponse BindResponse

	err := broker.ReSerializeInterface(requestParamsI, &bindResponse)
	if err != nil {
		return false
	}

	return bindResponse.DbName == p.param.DbName
}

func (p *perInstanceDatabasePlan) EqualBinding(bindInstance *broker.BindInstance, bindRequest *broker.BindRequest) bool {
	var bindParams newDatabaseBindParameters

	err := broker.ReSerializeInterface(bindRequest.Parameters, &bindParams)
	if err != nil {
		return false
	}
	var bindInstanceParams BindResponse
	err = broker.ReSerializeInterface(bindInstance.PlanParams, &bindInstanceParams)
	if err != nil {
		return false
	}

	return bindParams.Password == bindInstanceParams.Password && bindParams.Username == bindInstanceParams.Username
}
