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

// Catalog structures

// CatalogResponse is the top level object for the document returned to
// the client when it requests a catalog of services offered by this
// this service broker.
type CatalogResponse struct {
	Services []CatalogService `json:"services"`
}

// CatalogService is a catalog entry that describes one of the brokers
// service offerings.  This broker only offers a single service.
type CatalogService struct {
	Name           string        `json:"name"`
	ID             string        `json:"id"`
	Description    string        `json:"description"`
	Bindable       bool          `json:"bindable"`
	PlanUpdateable bool          `json:"plan_updateable, omitempty"`
	Plans          []ServicePlan `json:"plans"`
}

// ServicePlan is a catalog entry that describes a plan.  Currently the
// only plan offered by this service is the shared plan.
type ServicePlan struct {
	Name        string      `json:"name"`
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Metadata    interface{} `json:"metadata,omitempty"`
	Free        bool        `json:"free,omitempty"`
	Bindable    bool        `json:"bindable,omitempty"`
}

// Request structures

// CreateServiceInstanceRequest is the object representation of the clients
// request to create a new service instance.
type CreateServiceInstanceRequest struct {
	ServiceID        string      `json:"service_id"`
	PlanID           string      `json:"plan_id"`
	OrganizationGUID string      `json:"organization_guid"`
	SpaceGUID        string      `json:"space_guid"`
	Parameters       interface{} `json:"parameters, omitempty"`
}

// BindRequest is the object representation of the clients request to bind
// and application to a service instance.
type BindRequest struct {
	ServiceID  string       `json:"service_id"`
	PlanID     string       `json:"plan_id"`
	Resource   BindResource `json:"bind_resource, omitempty"`
	Parameters interface{}  `json:"parameters, omitempty"`
}

// BindResource describes that application being bound.
type BindResource struct {
	AppGUID string `json:"app_GUID, omitempty"`
}

// Response structures

// CreateGetServiceInstanceResponse is the response to the client
// when a service is created or looked up.
type CreateGetServiceInstanceResponse struct {
	DashboardURL  string         `json:"dashboard_url, omitempty"`
	LastOperation *LastOperation `json:"last_operation, omitempty"`
}

// LastOperation is used for async messaging.
type LastOperation struct {
	State                    string `json:"state"`
	Description              string `json:"description"`
	AsyncPollIntervalSeconds int    `json:"async_poll_interval_seconds, omitempty"`
}

// ErrorMessageResponse wraps up error messages that are sent to
// the client.
type ErrorMessageResponse struct {
	Description string `json:"description, omitempty"`
}

// BindResponse is the data sent back to the client after a bind.  The
// structure of Credentials is defined by the plan in use.
type BindResponse struct {
	Credentials interface{} `json:"credentials"`
}

// UnbindResponse is the empty reply to a successful unbind call.
type UnbindResponse struct {
}

// Internal structures

// ServiceInstance is the brokers representation of a service instance
// and contains and interface to the plan in use.  It can be serialized
// by Stores.
type ServiceInstance struct {
	Plan             Plan        `json:"-"`
	InstanceGUID     string      `json:"instance_guid"`
	PlanID           string      `json:"plan_id"`
	OrganizationGUID string      `json:"organization_guid"`
	SpaceGUID        string      `json:"space_guid"`
	ServiceID        string      `json:"service_id"`
	InstanceParams   interface{} `json:"plan_params"`
}

// BindInstance is used to represent bounded applications.  The PlanParams
// field is defined by the plan in use.  It can be serialized by a Store.
type BindInstance struct {
	BindGUID   string      `json:"binding_guid"`
	PlanParams interface{} `json:"plan_params"`
}

// DatabaseCredentials is a convenience object for passing around the
// credentials needed to access a Stardog service.
type DatabaseCredentials struct {
	Password string `json:"password"`
	Username string `json:"username"`
}

// Configuration Structures

// ServerConfig the configuration document that is passed to the broker
// when it is started.  It contains plan and storage information.
type ServerConfig struct {
	Port           string        `json:"port"`
	Plans          []PlanConfig  `json:"plans"`
	Storage        StorageConfig `json:"storage"`
	BrokerUsername string        `json:"broker_username"`
	BrokerPassword string        `json:"broker_password"`
	BrokerID       string        `json:"broker_id"`
	LogLevel       string        `json:"log_level"`
	LogFile        string        `json:"log_file"`
}

// PlanConfig contains the configuration information for a given plan.  The
// plan is looked up via PlanID and if found Parameters are passed to that
// plan module for processing.
type PlanConfig struct {
	PlanName   string      `json:"name"`
	PlanID     string      `json:"id"`
	Parameters interface{} `json:"parameters"`
}

// StorageConfig describes the storage module to be used with this instance
// of the storage broker.  Currently there is an in memory store and a Stardog
// store.
type StorageConfig struct {
	Type       string      `json:"type"`
	Parameters interface{} `json:"parameters"`
}

// VCAPService is used to decode a cloud foundry environment.
type VCAPService struct {
	Credentials map[string]interface{} `json:"credentials"`
	Label       string                 `json:"label"`
	Plan        string                 `json:"plan"`
	Name        string                 `json:"name"`
	Tags        []string               `json:"tags"`
}
