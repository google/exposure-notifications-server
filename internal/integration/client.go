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

// Package integration contains EN Server integration tests.
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

// EnServerClient provides Exposure Notifications API client to support integration testing.
type EnServerClient struct {
	client *http.Client
}

func (server EnServerClient) getRequest(url string) (*http.Response, error) {
	resp, err := server.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return unwrapResponse(resp)
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
	return unwrapResponse(resp)
}

func (server EnServerClient) PublishKeys(t *testing.T, request verifyapi.Publish) {
	resp, err := server.postRequest("/publish", request)
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	log.Printf("response: %v", resp.Status)
	t.Logf("Publish request is sent to %v", "/publish")
}

func (server EnServerClient) CleanupExposures(t *testing.T) {
	resp, err := server.getRequest("/cleanup-exposure")
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	t.Logf("Cleanup exposures request is sent to /cleanup-exposure")
}

func (server EnServerClient) CleanupExports(t *testing.T) {
	resp, err := server.getRequest("/cleanup-export")
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	t.Logf("Cleanup exports request is sent to /cleanup-export")
}

func (server EnServerClient) ExportBatches(t *testing.T) {
	resp, err := server.getRequest("/export/create-batches")
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	t.Logf("Create batches request is sent to %v", "/export/create-batches")
}

func (server EnServerClient) StartExportWorkers(t *testing.T) {
	resp, err := server.getRequest("/export/do-work")
	if err != nil {
		t.Fatalf("request failed: %v, %v", err, resp)
	}
	t.Logf("Export worker request is sent to %v", "/export/do-work")
}

func unwrapResponse(resp *http.Response) (*http.Response, error) {
	if resp.StatusCode != http.StatusOK {
		// Return error upstream.
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to copy error body (%d): %w", resp.StatusCode, err)
		}
		return resp, fmt.Errorf("request failed with status: %v\n%v", resp.StatusCode, string(body))
	}
	return resp, nil
}
