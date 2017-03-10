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

// Package broker implements the HTTP handlers for the Stardog Service
// Broker
package broker

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// The ControllerImpl is the object that contains the handler functions for
// the service broker
type ControllerImpl struct {
	databasePlanMap map[string]PlanFactory
	logger          SdLogger
	store           Store
	stardogURL      string
	adminName       string
	adminPw         string
	brokerName      string
	brokerPw        string
	BrokerID        string
	clientFactory   StardogClientFactory
}

// CreateController makes a ControllerImpl object and returns it as a Controller interface
// to the main thread.
func CreateController(databasePlanMap map[string]PlanFactory, conf *ServerConfig, clientFactory StardogClientFactory, logger SdLogger, store Store) (Controller, error) {
	logger.Logf(INFO, "Creating a controller using configuration %s", conf)

	return &ControllerImpl{
		databasePlanMap: databasePlanMap,
		logger:          logger,
		store:           store,
		brokerName:      conf.BrokerUsername,
		brokerPw:        conf.BrokerPassword,
		BrokerID:        conf.BrokerID,
		clientFactory:   clientFactory,
	}, nil
}

// Catalog returns the information describing what this service broker offers.
func (c *ControllerImpl) Catalog(w http.ResponseWriter, r *http.Request) {
	c.logger.Logf(INFO, "Catalog called")
	err := HTTPBasicCheck(r, w, c.brokerName, c.brokerPw)
	if err != nil {
		c.logger.Logf(INFO, "Authorization failed %s", err)
		return
	}

	catalogService := CatalogService{
		Name:           "Stardog",
		ID:             c.BrokerID,
		Description:    "Provides access to a Stardog database",
		Bindable:       true,
		PlanUpdateable: false,
	}

	servicePlans := make([]ServicePlan, len(c.databasePlanMap))
	i := 0
	for _, v := range c.databasePlanMap {
		servicePlans[i] = ServicePlan{
			Name:        v.PlanName(),
			ID:          v.PlanID(),
			Description: v.PlanDescription(),
			Metadata:    v.Metadata(),
			Free:        v.Free(),
			Bindable:    v.Bindable(),
		}
		i++
	}
	catalogService.Plans = servicePlans

	var catalogResponse CatalogResponse
	catalogResponse.Services = make([]CatalogService, 1)
	catalogResponse.Services[0] = catalogService

	c.logger.Logf(INFO, "Catalog returned %s", catalogResponse)
	WriteResponse(w, http.StatusOK, &catalogResponse)
}

// CreateServiceInstance is called when a client attempts to create a new instance.
func (c *ControllerImpl) CreateServiceInstance(w http.ResponseWriter, r *http.Request) {
	c.logger.Logf(INFO, "Create Service called")
	err := HTTPBasicCheck(r, w, c.brokerName, c.brokerPw)
	if err != nil {
		c.logger.Logf(INFO, "Authorization failed %s", err)
		return
	}

	var serviceRequest CreateServiceInstanceRequest
	err = ReadRequestBody(r, &serviceRequest)
	if err != nil {
		SendInternalError(w)
		return
	}
	serviceInstanceGUID, err := GetRouteVariable(r, "service_instance_GUID")
	c.logger.Logf(INFO, "Creating SERVICE %s\n", serviceRequest)
	if err != nil {
		SendError(c.logger, w, http.StatusBadRequest, "service_instance_GUID is required")
		return
	}
	planFactory := c.databasePlanMap[serviceRequest.PlanID]
	if planFactory == nil {
		SendError(c.logger, w, http.StatusBadRequest, fmt.Sprintf("%s is not a known plan", serviceRequest.PlanID))
		return
	}
	plan := planFactory.MakePlan(c.clientFactory, c.logger)

	params, err := json.Marshal(serviceRequest.Parameters)
	if err != nil {
		SendError(c.logger, w, http.StatusInternalServerError, err.Error())
		return
	}
	existinSi, err := getServiceInstance(c, serviceInstanceGUID)
	if existinSi != nil {
		if compareService(existinSi, &serviceRequest) {
			WriteResponse(w, http.StatusOK, CreateGetServiceInstanceResponse{})
		} else {
			SendError(c.logger, w, http.StatusConflict, fmt.Sprintf("%s already exists with different values", serviceInstanceGUID))
		}
		return
	}

	code, response, err := plan.CreateServiceInstance(params)
	if err != nil {
		SendError(c.logger, w, code, err.Error())
		return
	}

	si := &ServiceInstance{
		Plan:             plan,
		PlanID:           planFactory.PlanID(),
		InstanceGUID:     serviceInstanceGUID,
		InstanceParams:   response,
		OrganizationGUID: serviceRequest.OrganizationGUID,
		SpaceGUID:        serviceRequest.SpaceGUID,
		ServiceID:        serviceRequest.ServiceID,
	}
	c.logger.Logf(DEBUG, "Adding instance to the store.")
	err = c.store.AddInstance(serviceInstanceGUID, si)
	if err != nil {
		// XXX TODO Here we have to clean up the created service
		c.logger.Logf(ERROR, "Failed to add the instance to the store.  Resources leaked. %s", err)
		SendError(c.logger, w, http.StatusInternalServerError, err.Error())
		return
	}
	c.logger.Logf(INFO, "Created Service Instance %s", serviceInstanceGUID)
	WriteResponse(w, code, CreateGetServiceInstanceResponse{})
}

// GetServiceInstance looks up a service instance and returns information about
// the instance if it is found.
func (c *ControllerImpl) GetServiceInstance(w http.ResponseWriter, r *http.Request) {
	c.logger.Logf(INFO, "Get Service called")
	err := HTTPBasicCheck(r, w, c.brokerName, c.brokerPw)
	if err != nil {
		c.logger.Logf(INFO, "Authorization failed %s", err)
		return
	}

	serviceInstanceGUID, err := GetRouteVariable(r, "service_instance_GUID")
	if err != nil {
		SendError(c.logger, w, http.StatusBadRequest, "service_instance_GUID is required")
		return
	}
	c.logger.Logf(DEBUG, "Getting service %s\n", serviceInstanceGUID)
	instance, err := getServiceInstance(c, serviceInstanceGUID)
	if err != nil {
		c.logger.Logf(INFO, "Failed to get the instance %s", serviceInstanceGUID)
		SendError(c.logger, w, http.StatusNotFound, fmt.Sprintf("The service with ID %s was not found", serviceInstanceGUID))
		return
	}
	WriteResponse(w, http.StatusOK, instance)
}

// RemoveServiceInstance deletes a service instance and all of its bound applications.
func (c *ControllerImpl) RemoveServiceInstance(w http.ResponseWriter, r *http.Request) {
	c.logger.Logf(INFO, "Remove Service called")
	err := HTTPBasicCheck(r, w, c.brokerName, c.brokerPw)
	if err != nil {
		c.logger.Logf(INFO, "Authorization failed %s", err)
		return
	}

	serviceInstanceGUID, err := GetRouteVariable(r, "service_instance_GUID")
	if err != nil {
		SendError(c.logger, w, http.StatusBadRequest, "service_instance_GUID is required")
		return
	}
	serviceInstance, err := getServiceInstance(c, serviceInstanceGUID)
	if err != nil {
		SendError(c.logger, w, http.StatusGone, fmt.Sprintf("service_instance_GUID %s does not exist", serviceInstanceGUID))
		return
	}

	bindMap, err := c.store.GetAllBindings(serviceInstanceGUID)
	if err != nil {
		SendError(c.logger, w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, bind := range bindMap {
		_, err := serviceInstance.Plan.UnBind(bind.PlanParams)
		if err != nil {
			c.logger.Logf(ERROR, "Failed to clean up the binding %s", err)
		}
	}

	code, response, err := serviceInstance.Plan.RemoveInstance()
	if err != nil {
		c.logger.Logf(ERROR, "Error removing the service %s", err)
		SendError(c.logger, w, code, err.Error())
		return
	}
	c.store.DeleteInstance(serviceInstanceGUID)
	c.logger.Logf(INFO, "Removed Service %s", serviceInstanceGUID)
	WriteResponse(w, code, response)
}

// Bind associates an application with a service instance.  The Plan object
// used by the service does the work of the Bind and then the Store object
// persists information about the bind.
func (c *ControllerImpl) Bind(w http.ResponseWriter, r *http.Request) {
	c.logger.Logf(INFO, "Bind called")
	err := HTTPBasicCheck(r, w, c.brokerName, c.brokerPw)
	if err != nil {
		c.logger.Logf(INFO, "Authorization failed %s", err)
		return
	}

	serviceInstanceGUID, err := GetRouteVariable(r, "service_instance_GUID")
	if err != nil {
		SendError(c.logger, w, http.StatusBadRequest, "service_instance_GUID is required")
		return
	}
	serviceBindingGUID, err := GetRouteVariable(r, "service_binding_GUID")
	if err != nil {
		SendError(c.logger, w, http.StatusBadRequest, "service_binding_GUID is required")
		return
	}
	var bindRequest BindRequest
	err = ReadRequestBody(r, &bindRequest)
	if err != nil {
		SendInternalError(w)
		return
	}

	c.logger.Logf(INFO, "Attempting to bind %s %s", serviceInstanceGUID, serviceBindingGUID)
	serviceInstance, err := getServiceInstance(c, serviceInstanceGUID)
	if err != nil {
		SendError(c.logger, w, http.StatusInternalServerError, err.Error())
		return
	}
	if serviceInstance == nil {
		SendError(c.logger, w, http.StatusBadRequest, fmt.Sprintf("service_instance_GUID %s does not exist", serviceBindingGUID))
		return
	}

	params, err := json.Marshal(bindRequest.Parameters)
	if err != nil {
		SendError(c.logger, w, http.StatusInternalServerError, err.Error())
		return
	}

	serviceBinding, err := c.store.GetBinding(serviceInstanceGUID, serviceBindingGUID)
	if serviceBinding != nil {
		if serviceInstance.Plan.EqualBinding(serviceBinding, &bindRequest) {
			WriteResponse(w, http.StatusOK, CreateGetServiceInstanceResponse{})
		} else {
			SendError(c.logger, w, http.StatusConflict, fmt.Sprintf("%s already exists with different values", serviceBindingGUID))
		}
		return
	}

	code, response, err := serviceInstance.Plan.Bind(serviceInstance.InstanceParams, params)
	if err != nil {
		SendError(c.logger, w, code, err.Error())
		return
	}
	bindInstance := BindInstance{
		PlanParams: response,
		BindGUID:   serviceBindingGUID,
	}
	bindResponse := &BindResponse{Credentials: response}

	err = c.store.AddBinding(serviceInstanceGUID, serviceBindingGUID, &bindInstance)
	if err != nil {
		// Need to undo the bind here
		c.logger.Logf(ERROR, "Failed to add the binding to the database.  Resources leaked. %s", err)
		SendError(c.logger, w, code, err.Error())
		return
	}

	c.logger.Logf(INFO, "Bound %s %s", serviceInstanceGUID, serviceBindingGUID)
	WriteResponse(w, http.StatusCreated, bindResponse)
}

// UnBind removes an application's association with a service instance by calling
// into the Plan module and then instructing the Store module to delete its
// association.
func (c *ControllerImpl) UnBind(w http.ResponseWriter, r *http.Request) {
	c.logger.Logf(INFO, "Unbind called")
	err := HTTPBasicCheck(r, w, c.brokerName, c.brokerPw)
	if err != nil {
		c.logger.Logf(INFO, "Authorization failed %s", err)
		return
	}

	serviceInstanceGUID, err := GetRouteVariable(r, "service_instance_GUID")
	if err != nil {
		SendError(c.logger, w, http.StatusBadRequest, "service_instance_GUID is required")
		return
	}
	serviceBindingGUID, err := GetRouteVariable(r, "service_binding_GUID")
	if err != nil {
		SendError(c.logger, w, http.StatusBadRequest, "service_binding_GUID is required")
		return
	}

	c.logger.Logf(INFO, "Attempting to unbind instance %s binding %s", serviceInstanceGUID, serviceBindingGUID)
	serviceInstance, err := getServiceInstance(c, serviceInstanceGUID)
	if err != nil {
		SendError(c.logger, w, http.StatusGone, fmt.Sprintf("service_instance_GUID %s does not exist", serviceInstanceGUID))
		return
	}
	serviceBinding, err := c.store.GetBinding(serviceInstanceGUID, serviceBindingGUID)
	if err != nil {
		SendError(c.logger, w, http.StatusGone, fmt.Sprintf("service_binding_GUID %s does not exist", serviceBindingGUID))
		return
	}

	err = c.store.DeleteBinding(serviceInstanceGUID, serviceBindingGUID)
	if err != nil {
		SendError(c.logger, w, http.StatusInternalServerError, err.Error())
		return
	}

	code, err := serviceInstance.Plan.UnBind(serviceBinding.PlanParams)
	if err != nil {
		SendError(c.logger, w, http.StatusInternalServerError, err.Error())
		return
	}
	c.logger.Logf(INFO, "Unbound %s %s", serviceInstanceGUID, serviceBindingGUID)
	WriteResponse(w, code, &UnbindResponse{})
}

func getServiceInstance(c *ControllerImpl, serviceGUID string) (*ServiceInstance, error) {
	serviceInstance, err := c.store.GetInstance(serviceGUID)
	if err != nil {
		return nil, err
	}
	pf, ok := c.databasePlanMap[serviceInstance.PlanID]
	if !ok {
		return nil, fmt.Errorf("The reported plan %s is unknown", serviceInstance.PlanID)
	}
	p, err := pf.InflatePlan(serviceInstance, c.clientFactory, c.logger)
	if err != nil {
		return nil, err
	}
	serviceInstance.Plan = p
	serviceInstance.PlanID = p.PlanID()

	return serviceInstance, nil
}

func compareService(serviceInstance *ServiceInstance, serviceRequest *CreateServiceInstanceRequest) bool {
	if serviceInstance.OrganizationGUID != serviceRequest.OrganizationGUID ||
		serviceInstance.SpaceGUID != serviceRequest.SpaceGUID ||
		serviceInstance.PlanID != serviceRequest.PlanID ||
		serviceInstance.ServiceID != serviceRequest.ServiceID {
		return false
	}
	// Compare Parameters
	return serviceInstance.Plan.EqualInstance(serviceRequest.Parameters)
}
