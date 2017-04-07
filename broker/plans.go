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

// PlanFactory holds the information needed to create a plan instance. When
// the instance is new MakePlan is used.  To inflate an existing instance
// InflatePlan is used.
type PlanFactory interface {
	PlanName() string
	PlanDescription() string
	PlanID() string
	Metadata() interface{}
	Free() bool
	Bindable() bool
	InflatePlan(interface{}, StardogClientFactory, SdLogger) (Plan, error)
}

// Plan represents a Plan that is associated with the service instance and
// application bindings.
type Plan interface {
	CreateServiceInstance() (int, interface{}, error)
	RemoveInstance() (int, interface{}, error)
	Bind(interface{}) (int, interface{}, error)
	UnBind(interface{}) (int, error)
	PlanID() string
	EqualInstance(interface{}) bool
	EqualBinding(*BindInstance, *BindRequest) bool
}
