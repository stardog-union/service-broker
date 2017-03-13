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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

type stardogClientImpl struct {
	sdURL   string
	dbCreds DatabaseCredentials
	logger  SdLogger
}

type sdRealClientFactory struct {
	logger SdLogger
}

// NewClientFactory returns an object that will create StardogClient objects that
// interact with a Stardog service
func NewClientFactory(logger SdLogger) StardogClientFactory {
	return &sdRealClientFactory{logger: logger}
}

// GetStardogAdminClient generates a stardog client object.  This gives a hook for mock objects
// in testing.
func (f *sdRealClientFactory) GetStardogAdminClient(sdURL string, dbCreds DatabaseCredentials) StardogClient {
	client := stardogClientImpl{sdURL: sdURL, dbCreds: dbCreds, logger: f.logger}
	return &client
}

// NewStardogClient creates a StardogClient network API object
func NewStardogClient(sdURL string, dbCreds DatabaseCredentials, logger SdLogger) StardogClient {
	s := stardogClientImpl{
		sdURL:   sdURL,
		dbCreds: dbCreds,
		logger:  logger,
	}
	return &s
}

func (s *stardogClientImpl) CreateDatabase(dbName string) error {
	data := fmt.Sprintf("{\"dbname\": \"%s\", \"options\" : {}, \"files\": []}", dbName)
	s.logger.Logf(DEBUG, "Creating the database with %s\n", data)

	dbURL := fmt.Sprintf("%s/admin/databases", s.sdURL)
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	err := bodyWriter.WriteField("root", data)
	if err != nil {
		return fmt.Errorf("didnt make write field %s", err)
	}
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	req, err := http.NewRequest("POST", dbURL, bodyBuf)
	if err != nil {
		return fmt.Errorf("Failed to create the req %s url %s", dbURL, err)
	}
	req.SetBasicAuth(s.dbCreds.Username, s.dbCreds.Password)

	client := &http.Client{}
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Logf(DEBUG, "Failed to connect to the database %s\n", err)
		return fmt.Errorf("Failed do the post %s", err)
	}
	if resp.StatusCode != 201 {
		return fmt.Errorf("Failed to create the database")
	}
	return nil
}

func (s *stardogClientImpl) GetDatabaseSize(dbName string) (int, error) {
	s.logger.Logf(DEBUG, "GetDatabase the database %s\n", dbName)

	dbURL := fmt.Sprintf("%s/%s/size", s.sdURL, dbName)
	bodyBuf := &bytes.Buffer{}
	content, err := s.doRequest("GET", dbURL, bodyBuf, "text/plain", 200)
	if err != nil {
		return -1, err
	}
	i, err := strconv.Atoi(string(content))
	return i, err
}

func (s *stardogClientImpl) AddData(dbName string, format string, data string) error {
	dbURL := fmt.Sprintf("%s/%s/transaction/begin", s.sdURL, dbName)
	bodyBuf := &bytes.Buffer{}
	content, err := s.doRequest("POST", dbURL, bodyBuf, "text/plain", 200)
	if err != nil {
		return err
	}
	txID := string(content)

	dataPayload := strings.NewReader(string(data))
	dbURL = fmt.Sprintf("%s/%s/%s/add", s.sdURL, dbName, txID)
	_, err = s.doRequestWithAccept("POST", dbURL, dataPayload, format, "text/plain", 200)
	if err != nil {
		return err
	}

	dbURL = fmt.Sprintf("%s/%s/transaction/commit/%s", s.sdURL, dbName, txID)
	_, err = s.doRequest("POST", dbURL, bodyBuf, "text/plain", 200)
	if err != nil {
		return err
	}
	return nil
}

func (s *stardogClientImpl) Query(dbName string, data string) ([]byte, error) {
	bodyBuf := &bytes.Buffer{}
	q := url.QueryEscape(data)
	dbURL := fmt.Sprintf("%s/%s/query?query=%s", s.sdURL, dbName, q)
	content, err := s.doRequestWithAccept("GET", dbURL, bodyBuf, "application/ld+json", "application/sparql-results+json", 200)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (s *stardogClientImpl) AddDocument(dbName string, doc string) error {
	dbURL := fmt.Sprintf("%s/%s/docs", s.sdURL, dbName)

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "upload", "docname6"))
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Transfer-Encoding", "binary")
	io, err := bodyWriter.CreatePart(h)
	if err != nil {
		return fmt.Errorf("didnt make write field %s", err)
	}
	contentType := bodyWriter.FormDataContentType()
	io.Write([]byte(doc))
	bodyWriter.Close()

	_, err = s.doRequest("POST", dbURL, bodyBuf, contentType, 201)
	if err != nil {
		s.logger.Logf(ERROR, "Adding the document failed %s", err)
		return err
	}
	return nil
}

type newUserRequest struct {
	Username  string   `json:"username"`
	Superuser bool     `json:"superuser"`
	Password  []string `json:"password"`
}

type userListResponse struct {
	Users []string `json:"users"`
}

func (s *stardogClientImpl) UserExists(username string) (bool, error) {
	dbURL := fmt.Sprintf("%s/admin/users", s.sdURL)
	bodyBuf := &bytes.Buffer{}
	content, err := s.doRequest("GET", dbURL, bodyBuf, "application/json", 200)
	if err != nil {
		return false, err
	}
	var users userListResponse
	err = json.Unmarshal(content, &users)
	if err != nil {
		return false, err
	}
	for _, u := range users.Users {
		if u == username {
			return true, nil
		}
	}
	return false, nil
}

func (s *stardogClientImpl) NewUser(username string, pw string) error {
	request := &newUserRequest{
		Username:  username,
		Superuser: false,
		Password:  make([]string, len(pw)),
	}
	for i := 0; i < len(pw); i++ {
		request.Password[i] = string(pw[i])
	}

	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	bodyBuf := strings.NewReader(string(data))

	dbURL := fmt.Sprintf("%s/admin/users", s.sdURL)
	c, err := s.doRequest("POST", dbURL, bodyBuf, "application/json", 201)
	if err != nil {
		s.logger.Logf(WARN, "Failed to create a new user %s %s", username, string(c))
		return err
	}
	return nil
}

type userPermissionDb struct {
	Action       string   `json:"action"`
	ResourceType string   `json:"resource_type"`
	Resource     []string `json:"resource"`
}

func (s *stardogClientImpl) GrantUserAccessToDb(dbName string, username string) error {
	dbURL := fmt.Sprintf("%s/admin/permissions/user/%s", s.sdURL, username)
	up := &userPermissionDb{
		Action:       "write",
		ResourceType: "db",
		Resource:     []string{dbName},
	}
	data, err := json.Marshal(up)
	if err != nil {
		return err
	}
	bodyBuf := strings.NewReader(string(data))
	_, err = s.doRequest("PUT", dbURL, bodyBuf, "application/json", 201)
	if err != nil {
		s.logger.Logf(ERROR, "Failed to set write permissions %s", err)
		return err
	}
	up.Action = "read"
	data, err = json.Marshal(up)
	if err != nil {
		return err
	}
	bodyBuf = strings.NewReader(string(data))
	_, err = s.doRequest("PUT", dbURL, bodyBuf, "application/json", 201)
	if err != nil {
		s.logger.Logf(ERROR, "Failed to set read permissions %s", err)
		return err
	}
	return nil
}

func (s *stardogClientImpl) doRequest(method, urlStr string, body io.Reader, contentType string, expectedCode int) ([]byte, error) {
	return s.doRequestWithAccept(method, urlStr, body, contentType, contentType, expectedCode)
}

func (s *stardogClientImpl) doRequestWithAccept(method, urlStr string, body io.Reader, contentType string, accept string, expectedCode int) ([]byte, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.dbCreds.Username, s.dbCreds.Password)
	client := &http.Client{}
	req.Header.Set("Content-Type", contentType)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed do the post %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedCode {
		return nil, fmt.Errorf("Expected %d but got %d when %s to %s", expectedCode, resp.StatusCode, method, urlStr)
	}
	content, err := ioutil.ReadAll(resp.Body)
	s.logger.Logf(INFO, "Completed %s to %s", method, urlStr)
	return content, nil
}

func (s *stardogClientImpl) doRequestResponse(method, urlStr string, body io.Reader, contentType string, expectedCode int) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.dbCreds.Username, s.dbCreds.Password)
	client := &http.Client{}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed do the post %s", err)
	}
	if resp.StatusCode != expectedCode {
		resp.Body.Close()
		return nil, fmt.Errorf("Expected %d but got %d when %s to %s", expectedCode, resp.StatusCode, method, urlStr)
	}
	return resp, nil
}

func (s *stardogClientImpl) DeleteUser(username string) error {
	s.logger.Logf(INFO, "Deleting the user %s", username)
	dbURL := fmt.Sprintf("%s/admin/users/%s", s.sdURL, username)
	bodyBuf := &bytes.Buffer{}
	c, err := s.doRequest("DELETE", dbURL, bodyBuf, "application/json", 200)
	if err != nil {
		s.logger.Logf(WARN, "Error deleting user %s %s", string(c), err)
		return err
	}
	return nil
}

func (s *stardogClientImpl) RevokeUserAccess(dbName string, username string) error {
	s.logger.Logf(INFO, "Revoking user %s access to %s", username, dbName)

	up := &userPermissionDb{
		Action:       "write",
		ResourceType: "db",
		Resource:     []string{dbName},
	}
	data, err := json.Marshal(up)
	if err != nil {
		return err
	}
	bodyBuf := strings.NewReader(string(data))

	dbURL := fmt.Sprintf("%s/admin/permissions/user/%s/delete", s.sdURL, username)
	c, err := s.doRequest("POST", dbURL, bodyBuf, "application/json", 200)
	if err != nil {
		s.logger.Logf(WARN, "Error revoking access %s %s", string(c), err)
		return err
	}
	up.Action = "read"
	data, err = json.Marshal(up)
	if err != nil {
		return err
	}
	bodyBuf = strings.NewReader(string(data))

	c, err = s.doRequest("POST", dbURL, bodyBuf, "application/json", 200)
	if err != nil {
		s.logger.Logf(WARN, "Error revoking access %s %s", string(c), err)
		return err
	}
	return nil
}

func (s *stardogClientImpl) DeleteDatabase(dbName string) error {
	s.logger.Logf(INFO, "Deleting the database %s", dbName)

	dbURL := fmt.Sprintf("%s/admin/databases/%s", s.sdURL, dbName)
	bodyBuf := &bytes.Buffer{}
	c, err := s.doRequest("DELETE", dbURL, bodyBuf, "application/json", 200)
	if err != nil {
		s.logger.Logf(WARN, "Error deleting the db %s %s", string(c), err)
		return err
	}
	return nil
}
