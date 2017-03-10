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
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type instanceWrapper struct {
	inst       *ServiceInstance
	bindingMap map[string]*BindInstance
}

type inMemoryStore struct {
	instanceMap map[string]*instanceWrapper
	logger      SdLogger
}

type stardogStore struct {
	client *stardogClientImpl
	logger SdLogger
	dbName string
}

type jsonReply struct {
	Head    map[string][]string `json:"head"`
	Results results             `json:"results"`
}

type results struct {
	Bindings []map[string]sdReplyBinding `json:"bindings"`
}

type sdReplyBinding struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type boolReply struct {
	Head    map[string][]string `json:"head"`
	Boolean bool                `json:"boolean"`
}

// NewInMemoryStore creates a Store object that only keeps information
// in main memory.  This is used for testing.
func NewInMemoryStore(logger SdLogger) Store {
	return &inMemoryStore{
		instanceMap: make(map[string]*instanceWrapper),
		logger:      logger,
	}
}

type stardogMetadataStore struct {
	StardogURL string `json:"stardog_url"`
	AdminName  string `json:"admin_username"`
	AdminPw    string `json:"admin_password"`
}

// NewStardogStore creates a Store object that will persist the broker information to a
// Stardog database.
func NewStardogStore(BrokerID string, logger SdLogger, parameters interface{}) (Store, error) {
	var sdStoreParameters stardogMetadataStore
	err := ReSerializeInterface(parameters, &sdStoreParameters)
	if err != nil {
		return nil, err
	}

	logger.Logf(DEBUG, "Setting up persist with params %s", parameters)
	logger.Logf(DEBUG, "Setting up persist with @@ %s", sdStoreParameters)

	// Create database for storing instance info
	client := stardogClientImpl{
		sdURL: sdStoreParameters.StardogURL,
		dbCreds: DatabaseCredentials{
			Username: sdStoreParameters.AdminName,
			Password: sdStoreParameters.AdminPw,
		},
		logger: logger,
	}
	sdStore := stardogStore{
		client: &client,
		logger: logger,
	}
	sdStore.dbName = fmt.Sprintf("metadata%s", BrokerID)
	_, err = client.GetDatabaseSize(sdStore.dbName)
	if err != nil {
		logger.Logf(INFO, "The database %s does not exist.  Try making it: %s|", sdStore.dbName, client.sdURL)
		err := client.CreateDatabase(sdStore.dbName)
		if err != nil {
			return nil, err
		}
	}

	return &sdStore, nil
}

func (s *stardogStore) AddInstance(id string, instance *ServiceInstance) error {
	instanceData, err := json.Marshal(instance)
	if err != nil {
		return nil
	}
	encodedData := base64.StdEncoding.EncodeToString(instanceData)

	insert := `@prefix sdcf: <http://github.com/stardog-union/service-broker/> .
	sdcf:instance%s sdcf:GUID "%s"^^xsd:string .
	sdcf:instance%s sdcf:isa sdcf:instance .
	sdcf:instance%s sdcf:datais "%s"^^xsd:string .`

	payload := fmt.Sprintf(insert, id, id, id, id, encodedData)
	err = s.client.AddData(s.dbName, "text/turtle", payload)

	return err
}

func (s *stardogStore) GetInstance(id string) (*ServiceInstance, error) {
	qS := `
	PREFIX sdcf: <http://github.com/stardog-union/service-broker/>

select ?instance_data where {
  ?instance sdcf:isa sdcf:instance .
  ?instance sdcf:GUID "%s"^^xsd:string .
  ?instance sdcf:datais ?instance_data .
}
	`
	q := fmt.Sprintf(qS, id)
	b, err := s.client.Query(s.dbName, q)
	if err != nil {
		return nil, err
	}

	var res jsonReply
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	if len(res.Results.Bindings) < 1 {
		return nil, fmt.Errorf("There was no instance data in the query results")
	}
	instanceData, ok := res.Results.Bindings[0]["instance_data"]
	if !ok {
		return nil, fmt.Errorf("There was no instance data in the query results")
	}
	siB, err := base64.StdEncoding.DecodeString(instanceData.Value)
	if err != nil {
		return nil, err
	}
	var si ServiceInstance
	err = json.Unmarshal(siB, &si)
	if err != nil {
		return nil, err
	}
	return &si, nil
}

func (s *stardogStore) DeleteInstance(id string) error {
	d := `PREFIX sdcf: <http://github.com/stardog-union/service-broker/>

	DELETE WHERE {
		sdcf:instance%s ?o ?p .
	}`

	_, err := s.client.Query(s.dbName, fmt.Sprintf(d, id))
	if err != nil {
		return err
	}

	return nil
}

func (s *stardogStore) GetAllBindings(instanceID string) (map[string]*BindInstance, error) {
	q := `PREFIX sdcf: <http://github.com/stardog-union/service-broker/>
select ?data_binding where {
  ?instance sdcf:isa sdcf:instance .
  ?instance sdcf:GUID "%s" .
  ?binding sdcf:isa sdcf:binding .
  ?binding sdcf:boundto ?instance .
  ?binding sdcf:datais ?data_binding .
}`
	b, err := s.client.Query(s.dbName, fmt.Sprintf(q, instanceID))
	if err != nil {
		return nil, err
	}
	var res jsonReply
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	outRes := make(map[string]*BindInstance)

	for _, v := range res.Results.Bindings {
		encodedEnt, ok := v["data_binding"]
		if !ok {
			return nil, fmt.Errorf("Bad protocol response")
		}
		decoded, err := base64.StdEncoding.DecodeString(encodedEnt.Value)
		if err != nil {
			return nil, err
		}
		var bi BindInstance
		err = json.Unmarshal(decoded, &bi)
		if err != nil {
			return nil, err
		}
		outRes[bi.BindGUID] = &bi
	}

	return outRes, nil
}

func (s *stardogStore) AddBinding(instanceID string, bindingID string, bindInstance *BindInstance) error {
	_, err := s.GetInstance(instanceID)
	if err != nil {
		return err
	}
	bindData, err := json.Marshal(bindInstance)
	if err != nil {
		return nil
	}
	encodedData := base64.StdEncoding.EncodeToString(bindData)

	insert := `@prefix sdcf: <http://github.com/stardog-union/service-broker/> .
	sdcf:binding%s sdcf:GUID "%s"^^xsd:string .
	sdcf:binding%s sdcf:isa sdcf:binding .
	sdcf:binding%s sdcf:datais "%s"^^xsd:string .
	sdcf:binding%s sdcf:boundto sdcf:instance%s .
	`
	payload := fmt.Sprintf(insert, bindingID, bindingID, bindingID, bindingID, encodedData, bindingID, instanceID)
	err = s.client.AddData(s.dbName, "text/turtle", payload)

	return err
}

func (s *stardogStore) DeleteBinding(instanceID string, bindingID string) error {
	d := `PREFIX sdcf: <http://github.com/stardog-union/service-broker/>

	delete where {
      		sdcf:binding%s ?o ?p .
      	}`

	r, err := s.client.Query(s.dbName, fmt.Sprintf(d, bindingID))
	if err != nil {
		return err
	}
	var res boolReply
	err = json.Unmarshal(r, &res)
	if err != nil {
		return err
	}
	if !res.Boolean {
		return fmt.Errorf("Failed to delete")
	}
	return nil
}

func (s *stardogStore) GetBinding(instanceID string, bindingID string) (*BindInstance, error) {
	q := `PREFIX sdcf: <http://github.com/stardog-union/service-broker/>
select ?data_binding where {
  ?instance sdcf:isa sdcf:instance .
  ?instance sdcf:GUID "%s" .
  ?binding sdcf:isa sdcf:binding .
  ?binding sdcf:boundto ?instance .
  ?binding sdcf:GUID "%s" .
  ?binding sdcf:datais ?data_binding .
}`
	b, err := s.client.Query(s.dbName, fmt.Sprintf(q, instanceID, bindingID))
	if err != nil {
		return nil, err
	}
	var res jsonReply
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	if len(res.Results.Bindings) < 1 {
		return nil, fmt.Errorf("There was no binding data in the query results")
	}
	v, ok := res.Results.Bindings[0]["data_binding"]
	if !ok {
		return nil, fmt.Errorf("There was no instance data in the query results")
	}
	decoded, err := base64.StdEncoding.DecodeString(v.Value)
	if err != nil {
		return nil, err
	}
	var bi BindInstance
	err = json.Unmarshal(decoded, &bi)
	if err != nil {
		return nil, err
	}
	return &bi, nil
}

func (m *inMemoryStore) AddInstance(id string, instance *ServiceInstance) error {
	inst := m.instanceMap[id]
	if inst != nil {
		return fmt.Errorf("The instance already exists")
	}
	w := &instanceWrapper{
		inst:       instance,
		bindingMap: make(map[string]*BindInstance),
	}
	m.instanceMap[id] = w
	m.logger.Logf(INFO, "Added instance %s", id)
	return nil
}

func (m *inMemoryStore) GetInstance(id string) (*ServiceInstance, error) {
	w := m.instanceMap[id]
	if w == nil {
		return nil, fmt.Errorf("The instance does not exists")
	}
	return w.inst, nil
}

func (m *inMemoryStore) DeleteInstance(id string) error {
	w := m.instanceMap[id]
	if w == nil {
		return fmt.Errorf("The instance does not exists")
	}
	delete(m.instanceMap, id)
	return nil
}

func (m *inMemoryStore) GetAllBindings(instanceID string) (map[string]*BindInstance, error) {
	w := m.instanceMap[instanceID]
	if w == nil {
		return nil, fmt.Errorf("The instance does not exists")
	}
	return w.bindingMap, nil
}

func (m *inMemoryStore) AddBinding(instanceID string, bindingID string, bindInstance *BindInstance) error {
	w := m.instanceMap[instanceID]
	if w == nil {
		return fmt.Errorf("The instance does not exists %s", instanceID)
	}
	b := w.bindingMap[bindingID]
	if b != nil {
		return fmt.Errorf("The binding already exists %s", bindingID)
	}
	m.logger.Logf(INFO, "Memory store binding %s %s", instanceID, bindingID)
	w.bindingMap[bindingID] = bindInstance
	return nil
}

func (m *inMemoryStore) DeleteBinding(instanceID string, bindingID string) error {
	w := m.instanceMap[instanceID]
	if w == nil {
		return fmt.Errorf("The instance does not exists %s", instanceID)
	}
	b := w.bindingMap[bindingID]
	if b == nil {
		return fmt.Errorf("The binding does not exists %s", bindingID)
	}
	m.logger.Logf(INFO, "Memory store deleted binding %s %s", instanceID, bindingID)
	delete(w.bindingMap, bindingID)
	return nil
}

func (m *inMemoryStore) GetBinding(instanceID string, bindingID string) (*BindInstance, error) {
	w := m.instanceMap[instanceID]
	if w == nil {
		return nil, fmt.Errorf("The instance does not exists")
	}
	b := w.bindingMap[bindingID]
	if b == nil {
		return nil, fmt.Errorf("The binding does not exists")
	}
	return b, nil
}
