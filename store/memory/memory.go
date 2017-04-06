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

package memory

import (
	"fmt"

	"github.com/stardog-union/service-broker/broker"
)

type instanceWrapper struct {
	inst       *broker.ServiceInstance
	bindingMap map[string]*broker.BindInstance
}

type inMemoryStore struct {
	instanceMap map[string]*instanceWrapper
	logger      broker.SdLogger
}

// NewInMemoryStore creates a Store object that only keeps information
// in main memory.  This is used for testing.
func NewInMemoryStore(logger broker.SdLogger) broker.Store {
	return &inMemoryStore{
		instanceMap: make(map[string]*instanceWrapper),
		logger:      logger,
	}
}

func (m *inMemoryStore) AddInstance(id string, instance *broker.ServiceInstance) error {
	inst := m.instanceMap[id]
	if inst != nil {
		return fmt.Errorf("The instance already exists")
	}
	w := &instanceWrapper{
		inst:       instance,
		bindingMap: make(map[string]*broker.BindInstance),
	}
	m.instanceMap[id] = w
	m.logger.Logf(broker.INFO, "Added instance %s", id)
	return nil
}

func (m *inMemoryStore) GetInstance(id string) (*broker.ServiceInstance, error) {
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

func (m *inMemoryStore) GetAllBindings(instanceID string) (map[string]*broker.BindInstance, error) {
	w := m.instanceMap[instanceID]
	if w == nil {
		return nil, fmt.Errorf("The instance does not exists")
	}
	return w.bindingMap, nil
}

func (m *inMemoryStore) AddBinding(instanceID string, bindingID string, bindInstance *broker.BindInstance) error {
	w := m.instanceMap[instanceID]
	if w == nil {
		return fmt.Errorf("The instance does not exists %s", instanceID)
	}
	b := w.bindingMap[bindingID]
	if b != nil {
		return fmt.Errorf("The binding already exists %s", bindingID)
	}
	m.logger.Logf(broker.INFO, "Memory store binding %s %s", instanceID, bindingID)
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
	m.logger.Logf(broker.INFO, "Memory store deleted binding %s %s", instanceID, bindingID)
	delete(w.bindingMap, bindingID)
	return nil
}

func (m *inMemoryStore) GetBinding(instanceID string, bindingID string) (*broker.BindInstance, error) {
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
