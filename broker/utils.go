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
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// LoadJSON creates an object from a JSON file.
func LoadJSON(obj interface{}, path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, obj)
	if err != nil {
		return err
	}
	return nil
}

// WriteResponse Send data to the client with an HTTP header.
func WriteResponse(w http.ResponseWriter, code int, object interface{}) {
	data, err := json.Marshal(object)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(code)
	_, err = fmt.Fprintf(w, string(data))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// ReadRequestBody is a convenience function for writing an HTTP response.
func ReadRequestBody(r *http.Request, object interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, object)
	if err != nil {
		return err
	}
	return nil
}

// SendInternalError sends a bodiless HTTP error message to the client.
func SendInternalError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	return
}

// SendError sends an error message to the client with a description.
func SendError(logger SdLogger, w http.ResponseWriter, code int, desc string) {
	logger.Logf(ERROR, "Sending the error message %d %s", code, desc)
	e := ErrorMessageResponse{Description: desc}
	WriteResponse(w, code, &e)
}

// GetRouteVariable pulls a variable out of the HTTP path that the client
// sent.
func GetRouteVariable(r *http.Request, varName string) (string, error) {
	val, ok := mux.Vars(r)[varName]
	if !ok {
		return "", fmt.Errorf("A required variable did not exist")
	}
	return val, nil
}

// GetRandomName is a convience function for creating random strings.
func GetRandomName(base string, n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return fmt.Sprintf("%s%s", base, string(b))
}

// HTTPBasicCheck checks the authentication information in a HTTP basic auth header.
func HTTPBasicCheck(r *http.Request, w http.ResponseWriter, username string, password string) error {
	authHeader := r.Header["Authorization"]
	if authHeader == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("The HTTP header did not contain the Authorization header")
	}
	sent := ""
	for _, s := range authHeader {
		fields := strings.Fields(string(s))
		if len(fields) == 2 && strings.ToLower(fields[0]) == "basic" {
			sent = fields[1]
		}
	}
	if sent == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("HTTP basic authentication was not enabled")
	}
	data, err := base64.StdEncoding.DecodeString(sent)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return err
	}
	sA := strings.SplitN(string(data), ":", 2)
	if len(sA) != 2 {
		w.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("The authentication challenge was not properly formatted")
	}
	if username != sA[0] {
		w.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("The username %s does not match %s", sA[0], username)
	}
	if password != sA[1] {
		w.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("The password does not match")
	}
	return nil
}

// ReSerializeInterface takes an already inflated JSON object (typically
// a map[string]interface{}) and re-serializes it to a specific object
// type.  This is basically a work around to how Go deals with JSON
// objects and allows us to have plugs that define their own JSON types.
func ReSerializeInterface(in interface{}, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, out)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
