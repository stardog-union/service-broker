package mysql

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	// Load the mysql driver
	_ "github.com/go-sql-driver/mysql"
	"github.com/stardog-union/service-broker/broker"
)

var (
	MYSQL_SERVICE_TYPE_ENV = "MYSQL_SERVICE_TYPE"
	MYSQL_SERVICE_NAME_ENV = "MYSQL_SERVICE_NAME"
	MYSQL_PLAN_NAME_ENV    = "MYSQL_PLAN_NAME"
)

type mysqlNewParameters struct {
	UseVCap       bool   `json:"use_cap"`
	ServiceType   string `json:"service_type"`
	ServiceName   string `json:"service_name"`
	PlanName      string `json:"plan_name"`
	SQLDriverName string `json:"sql_driver_name"`
	ContactString string `json:"contact_string"`
	DatabaseName  string `json:"database_name"`
}

type mysqlStore struct {
	contactString string
	dbConn        *sql.DB
	logger        broker.SdLogger
}

type serviceRow struct {
	id          int
	data        string
	serviceGUID string
}

func getNewParameters(logger broker.SdLogger, parameters interface{}) (*mysqlNewParameters, error) {
	var mysqlParams mysqlNewParameters
	err := broker.ReSerializeInterface(parameters, &mysqlParams)
	if err != nil {
		return nil, fmt.Errorf("Error parsing database parameters %s", err.Error())
	}
	return nil, nil
}

func createMetaDatabase(logger broker.SdLogger, driverName string, contactString string, dbName string) error {
	dbConn, err := sql.Open(driverName, contactString)
	if err != nil {
		return fmt.Errorf("Failed to connect to the database: %s", err)
	}
	defer dbConn.Close()

	logger.Logf(broker.DEBUG, "Create the database %s", dbName)
	_, err = dbConn.Exec("CREATE DATABASE IF NOT EXISTS " + dbName)
	if err != nil {
		return fmt.Errorf("failed to create to the database: %s", err)
	}

	logger.Logf(broker.DEBUG, "Set the database for use")
	_, err = dbConn.Exec("USE " + dbName)
	if err != nil {
		return fmt.Errorf("failed to set the database in use: %s", err)
	}

	serviceTable := `CREATE TABLE IF NOT EXISTS service_instance (
		id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		service_guid varchar(64) UNIQUE KEY,
		data TEXT
	)
	`
	logger.Logf(broker.DEBUG, "Create the service table")
	_, err = dbConn.Exec(serviceTable)
	if err != nil {
		return fmt.Errorf("Failed to create the service_instance table: %s", err)
	}
	bindTable := `CREATE TABLE IF NOT EXISTS bindings (
		id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		service_id INT,
		binding_guid varchar(64) UNIQUE KEY,
		data TEXT,
		FOREIGN KEY (service_id)
        	REFERENCES service_instance(id)
        	ON DELETE CASCADE
	)
	`
	logger.Logf(broker.DEBUG, "Create the bind table")
	_, err = dbConn.Exec(bindTable)
	if err != nil {
		return fmt.Errorf("Failed to create the service_instance table: %s", err)
	}
	return nil
}

func getValueSearchPath(defaultValue string, parameterVal string, envVarName string) string {
	value := defaultValue
	if parameterVal != "" {
		value = parameterVal
	}
	e := os.Getenv(envVarName)
	if e != "" {
		value = e
	}
	return value
}

func getVCAPPlan(logger broker.SdLogger, serviceType string, serviceName string, planType string) (string, string, error) {
	vcap, err := broker.GetVCAPServices()
	if err != nil {
		return "", "", err
	}

	logger.Logf(broker.DEBUG, "Parsing VCAP services for service type %s, service name %s, and plan type %s", serviceType, serviceName, planType)
	serviceType = getValueSearchPath("p-mysql", serviceType, MYSQL_SERVICE_TYPE_ENV)
	serviceName = getValueSearchPath("", serviceName, MYSQL_SERVICE_NAME_ENV)
	planType = getValueSearchPath("", planType, MYSQL_PLAN_NAME_ENV)

	serviceList, exists := vcap[serviceType]
	if !exists {
		logger.Logf(broker.ERROR, "Service %s not found", serviceType)
		return "", "", fmt.Errorf("No VCAP services exist")
	}
	logger.Logf(broker.DEBUG, "Found service type %s", serviceType)
	logger.Logf(broker.DEBUG, "Looking for service %s with plan %s", serviceName, planType)
	for _, service := range serviceList {
		logger.Logf(broker.DEBUG, "Found service %s with plan %s", service.Name, service.Plan)
		// If no specific name or plan is used we pick the first one
		if (serviceName == "" || service.Name == serviceName) &&
			(planType == "" || service.Plan == planType) {
			logger.Logf(broker.DEBUG, "Using the plan %s %s", service.Plan, service.Name)
			creds := service.Credentials
			username, ok := creds["username"].(string)
			if !ok {
				return "", "", fmt.Errorf("The username field in the VCAP_SERVICES is not a string")
			}
			password, ok := creds["password"].(string)
			if !ok {
				return "", "", fmt.Errorf("The password field in the VCAP_SERVICES is not a string")
			}
			hostname, ok := creds["hostname"].(string)
			if !ok {
				return "", "", fmt.Errorf("The hostname field in the VCAP_SERVICES is not a string")
			}
			dbName, ok := creds["name"].(string)
			if !ok {
				return "", "", fmt.Errorf("The name field in the VCAP_SERVICES is not a string")
			}
			port, ok := creds["port"].(float64)
			if !ok {
				return "", "", fmt.Errorf("The port field in the VCAP_SERVICES is not an integer %s", reflect.TypeOf(creds["port"]).Kind().String())
			}
			// username:password@protocol(address)/dbname?param=value
			uri := fmt.Sprintf("%s:%s@tcp(%s:%d)/", username, password, hostname, int(port))
			return uri, dbName, nil
		}
	}
	return "", "", fmt.Errorf("No matching service found in VCAP_SERVICES")
}

func NewMySQLStore(BrokerID string, logger broker.SdLogger, parameters interface{}) (broker.Store, error) {
	var mysqlParams mysqlNewParameters
	err := broker.ReSerializeInterface(parameters, &mysqlParams)
	if err != nil {
		return nil, fmt.Errorf("Error parsing database parameters %s", err.Error())
	}
	mysqlParams.SQLDriverName = strings.ToLower(strings.TrimSpace(mysqlParams.SQLDriverName))
	if mysqlParams.SQLDriverName == "" {
		mysqlParams.SQLDriverName = "mysql"
	}
	// For now we are only supporting mysql
	if mysqlParams.SQLDriverName != "mysql" {
		return nil, fmt.Errorf("The storage driver %s is not supported", mysqlParams.SQLDriverName)
	}

	var uri string
	var dbName string
	if mysqlParams.UseVCap {
		uri, dbName, err = getVCAPPlan(logger, mysqlParams.ServiceType, mysqlParams.ServiceName, mysqlParams.PlanName)
		if err != nil {
			return nil, err
		}
	} else {
		uri = mysqlParams.ContactString
		dbName = mysqlParams.DatabaseName
	}
	if dbName == "" {
		dbName = strings.Replace(BrokerID, "-", "", -1)
	}
	err = createMetaDatabase(logger, mysqlParams.SQLDriverName, uri, dbName)
	if err != nil {
		return nil, err
	}

	var m mysqlStore
	m.contactString = fmt.Sprintf("%s%s", uri, dbName)
	m.logger = logger
	m.dbConn, err = sql.Open(mysqlParams.SQLDriverName, m.contactString)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the database: %s", err)
	}
	return &m, nil
}

func (m *mysqlStore) AddInstance(serviceGUID string, instance *broker.ServiceInstance) error {
	instanceData, err := json.Marshal(instance)
	if err != nil {
		return nil
	}
	encodedData := base64.StdEncoding.EncodeToString(instanceData)

	stmt, err := m.dbConn.Prepare("INSERT INTO service_instance(service_guid, data) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("Failure to create the prepared statement: %s", err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(serviceGUID, encodedData)
	if err != nil {
		return fmt.Errorf("Failure to execute the insert: %s", err)
	}
	return nil
}

func (m *mysqlStore) GetInstance(serviceGUID string) (*broker.ServiceInstance, error) {
	tx, err := m.dbConn.Begin()
	defer tx.Commit()
	res, err := m.getServiceRow(tx, serviceGUID)
	if err != nil {
		return nil, err
	}
	siB, err := base64.StdEncoding.DecodeString(res.data)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode the base64 data: %s", err)
	}
	var si broker.ServiceInstance
	err = json.Unmarshal(siB, &si)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal the JSON: %s", err)
	}
	return &si, nil
}

func (m *mysqlStore) DeleteInstance(serviceGUID string) error {
	stmt, err := m.dbConn.Prepare("DELETE FROM service_instance WHERE service_guid = ?")
	if err != nil {
		return fmt.Errorf("Failure to create the prepared statement: %s", err)
	}
	defer stmt.Close()
	res, err := stmt.Exec(serviceGUID)
	if err != nil {
		return fmt.Errorf("Failure to execute the delete: %s", err)
	}
	c, err := res.RowsAffected()
	if c < 1 {
		return fmt.Errorf("No rows were deleted")
	}
	if c > 1 {
		m.logger.Logf(broker.WARN, "Multiple rows (%d) were deleted with the service id %s", c, serviceGUID)
	}
	return nil
}

func (m *mysqlStore) getServiceRow(tx *sql.Tx, serviceGUID string) (*serviceRow, error) {
	rows, err := tx.Query("select id, service_guid, data from service_instance where service_guid = ?", serviceGUID)
	if err != nil {
		return nil, fmt.Errorf("Failed to find the service instance: %s", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("The service instance %s was not found", serviceGUID)
	}
	var resultRow serviceRow
	err = rows.Scan(&resultRow.id, &resultRow.serviceGUID, &resultRow.data)
	if err != nil {
		return nil, fmt.Errorf("Failed to get the data for instance %s: %s", serviceGUID, err)
	}
	if rows.Next() {
		m.logger.Logf(broker.WARN, "Multiple rows found for %s", serviceGUID)
	}
	return &resultRow, nil
}

func (m *mysqlStore) AddBinding(serviceGUID string, bindingGUID string, bindInstance *broker.BindInstance) error {
	bindData, err := json.Marshal(bindInstance)
	if err != nil {
		return nil
	}
	encodedData := base64.StdEncoding.EncodeToString(bindData)

	tx, err := m.dbConn.Begin()
	res, err := m.getServiceRow(tx, serviceGUID)
	if err != nil {
		tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO bindings(service_id, binding_guid, data) VALUES (?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Failed to prepare the binding insert statement %s", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(res.id, bindingGUID, encodedData)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("Failed to execute the binding insert statement: %s", err)
	}
	return tx.Commit()
}

func (m *mysqlStore) GetBinding(serviceGUID string, bindingGUID string) (*broker.BindInstance, error) {
	rows, err := m.dbConn.Query("select data from bindings where binding_guid = ?", bindingGUID)
	if err != nil {
		return nil, fmt.Errorf("Failed to bind the binding id: %s", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("The service instance %s was not found", serviceGUID)
	}
	var data string
	err = rows.Scan(&data)
	if err != nil {
		return nil, fmt.Errorf("Failed to get the data for the binding %s: %s", bindingGUID, err)
	}
	if rows.Next() {
		m.logger.Logf(broker.WARN, "Multiple rows found for %s", serviceGUID)
	}
	biB, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode the base64 data: %s", err)
	}
	var bi broker.BindInstance
	err = json.Unmarshal(biB, &bi)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal the JSON: %s", err)
	}
	return &bi, nil
}

func (m *mysqlStore) GetAllBindings(serviceGUID string) (map[string]*broker.BindInstance, error) {
	tx, err := m.dbConn.Begin()
	res, err := m.getServiceRow(tx, serviceGUID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	rows, err := tx.Query("select data from bindings where service_id = ?", res.id)
	if err != nil {
		return nil, fmt.Errorf("Failed to find bingings for the service %s: %s", serviceGUID, err)
	}
	bindingMap := make(map[string]*broker.BindInstance)
	for rows.Next() {
		var data string
		err = rows.Scan(&data)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("Failed to get the bind value %s", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("Failed to decode the bind value %s", err)
		}
		var bi broker.BindInstance
		err = json.Unmarshal(decoded, &bi)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal the bind value %s", err)
		}
		bindingMap[bi.BindGUID] = &bi
	}
	return bindingMap, nil
}

func (m *mysqlStore) DeleteBinding(serviceGUID string, bindingGUID string) error {
	stmt, err := m.dbConn.Prepare("DELETE FROM bindings WHERE binding_guid = ?")
	if err != nil {
		return fmt.Errorf("Failure to create the prepared statement: %s", err)
	}
	defer stmt.Close()
	res, err := stmt.Exec(bindingGUID)
	if err != nil {
		return fmt.Errorf("Failure to execute the delete: %s", err)
	}
	c, err := res.RowsAffected()
	if c < 1 {
		return fmt.Errorf("No rows were deleted")
	}
	if c > 1 {
		m.logger.Logf(broker.WARN, "Multiple rows (%d) were deleted with the binding id %s", c, bindingGUID)
	}
	return nil
}
