// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

// Contains Exposure Notifications API client to support integration testing.
type EnServerClient struct {
	client *http.Client
}

// Posts requests to the specified url.
// This methods attempts to serialize data argument as a json.
func (server EnServerClient) postRequest(url string, data interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal json payload")
	}
	request := bytes.NewBuffer(jsonData)
	r, err := http.NewRequest("POST", url, request)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := server.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Return error upstream.
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to copy error body (%d): %w", resp.StatusCode, err)
		}
		return resp, fmt.Errorf("post request failed with status: %v\n%v", resp.StatusCode, body)
	}

	return resp, nil
}

func (server EnServerClient) PublishKeys(t *testing.T, request verifyapi.Publish) {
	resp, err := server.postRequest("/publish", request)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Publish request is sent to %v", "/publish")
}

func (server EnServerClient) ExportBatches(t *testing.T) {
	var bts []byte
	resp, err := server.postRequest("/export/create-batches", bts)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Create batches request is sent to %v", "/export/create-batches")
}

func (server EnServerClient) StartExportWorkers(t *testing.T) {
	var bts []byte
	resp, err := server.postRequest("/export/do-work", bts)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Export worker request is sent to %v", "/export/do-work")
}
