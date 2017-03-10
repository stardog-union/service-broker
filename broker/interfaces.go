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

import "net/http"

// StardogClientFactory creates netwrok API connection objects to a
// Stardog service.  This mainly serves and a place to insert mock objects
// for testing.
type StardogClientFactory interface {
	GetStardogAdminClient(string, DatabaseCredentials) StardogClient
}

// StardogClient is the object used to interact with the Stardog service.
// At some point it may make sense to break this out into its own package.
type StardogClient interface {
	CreateDatabase(string) error
	DeleteDatabase(string) error
	UserExists(string) (bool, error)
	NewUser(string, string) error
	DeleteUser(string) error
	GrantUserAccessToDb(string, string) error
	RevokeUserAccess(string, string) error
}

// Controller object handles the HTTP network API calls.
type Controller interface {
	Catalog(http.ResponseWriter, *http.Request)
	CreateServiceInstance(http.ResponseWriter, *http.Request)
	GetServiceInstance(http.ResponseWriter, *http.Request)
	RemoveServiceInstance(http.ResponseWriter, *http.Request)
	Bind(http.ResponseWriter, *http.Request)
	UnBind(http.ResponseWriter, *http.Request)
}

// Store is the interface to persisting information related to service instances
// and bounded applications.  There is an in memory store for testing and
// Stardog store for the shared plan.  More storage drivers maybe needed
// as more plans are created.
type Store interface {
	AddInstance(string, *ServiceInstance) error
	GetInstance(string) (*ServiceInstance, error)
	DeleteInstance(string) error
	AddBinding(string, string, *BindInstance) error
	GetBinding(string, string) (*BindInstance, error)
	GetAllBindings(string) (map[string]*BindInstance, error)
	DeleteBinding(string, string) error
}
