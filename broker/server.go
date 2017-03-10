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
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
)

// Server is the object that controlls running the HTTP server and the
// controller.  It is essentially an embedded service broker.
type Server struct {
	controller  Controller
	port        string
	server      http.Server
	listener    net.Listener
	doneChannel chan error
	logger      SdLogger
}

// CreateServer makes an instance of the BrokerServer.
func CreateServer(databasePlanMap map[string]PlanFactory, conf *ServerConfig, clientFactory StardogClientFactory, logger SdLogger, store Store) (*Server, error) {
	controller, err := CreateController(databasePlanMap, conf, clientFactory, logger, store)
	if err != nil {
		return nil, err
	}

	return &Server{
		controller: controller,
		port:       conf.Port,
		logger:     logger,
	}, nil
}

// Start begins listening for HTTP connections on a port.  The listening is doneChannel
// in a go routine and control is handed back to the calling thread.
func (s *Server) Start() error {
	var err error

	router := mux.NewRouter()

	router.HandleFunc("/v2/catalog", s.controller.Catalog).Methods("GET")
	router.HandleFunc("/v2/service_instances/{service_instance_GUID}", s.controller.GetServiceInstance).Methods("GET")
	router.HandleFunc("/v2/service_instances/{service_instance_GUID}", s.controller.CreateServiceInstance).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{service_instance_GUID}", s.controller.RemoveServiceInstance).Methods("DELETE")
	router.HandleFunc("/v2/service_instances/{service_instance_GUID}/service_bindings/{service_binding_GUID}", s.controller.Bind).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{service_instance_GUID}/service_bindings/{service_binding_GUID}", s.controller.UnBind).Methods("DELETE")

	s.server = http.Server{
		Addr:    ":" + s.port,
		Handler: router,
	}

	s.listener, err = net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	s.doneChannel = make(chan error)

	go func() {
		err = s.server.Serve(s.listener)
		s.doneChannel <- err
	}()
	return nil
}

// Wait will block on a running server until the Stop method is called.
func (s *Server) Wait() error {
	err := <-s.doneChannel
	return err
}

// Stop is called to stop Server object from listening.
func (s *Server) Stop(wait bool) error {
	err := s.listener.Close()
	if err != nil {
		return err
	}
	if wait {
		err = s.Wait()
	}
	return err
}

// GetAddr returns the addr object on which the server is listening.
func (s *Server) GetAddr() (net.Addr, error) {
	if s.listener == nil {
		return nil, fmt.Errorf("The server is not yet listening")
	}
	return s.listener.Addr(), nil
}
