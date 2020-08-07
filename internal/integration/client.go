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
	"net/http"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

// Client provides Exposure Notifications API client to support
// integration testing.
type Client struct {
	client *http.Client
}

func (c *Client) PublishKeys(payload *verifyapi.Publish) (*verifyapi.PublishResponse, error) {
	j, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	resp, err := c.client.Post("/publish", "application/json", bytes.NewReader(j))
	if err != nil {
		return nil, fmt.Errorf("failed to POST /publish: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /publish: %w: %s", err, body)
	}

	var pubResponse verifyapi.PublishResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return nil, fmt.Errorf("bad publish response")
	}

	return &pubResponse, nil
}

func (c *Client) CleanupExposures() error {
	resp, err := c.client.Get("/cleanup-exposure")
	if err != nil {
		return fmt.Errorf("failed to GET /cleanup-exposure: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return fmt.Errorf("failed to GET /cleanup-exposure: %w: %s", err, body)
	}
	return nil
}

func (c *Client) CleanupExports() error {
	resp, err := c.client.Get("/cleanup-export")
	if err != nil {
		return fmt.Errorf("failed to GET /cleanup-export: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return fmt.Errorf("failed to GET /cleanup-export: %w: %s", err, body)
	}
	return nil
}

func (c *Client) ExportBatches() error {
	resp, err := c.client.Get("/export/create-batches")
	if err != nil {
		return fmt.Errorf("failed to GET /export/create-batches: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return fmt.Errorf("failed to GET /export/create-batches: %w: %s", err, body)
	}
	return nil
}

func (c *Client) RotateKeys() error {
	resp, err := c.client.Get("/key-rotation/rotate-keys")
	if err != nil {
		return fmt.Errorf("failed to GET /key-rotation/rotate-keys: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return fmt.Errorf("failed to GET /key-rotation/rotate-keys: %w: %s", err, body)
	}
	return nil
}

func (c *Client) StartExportWorkers() error {
	resp, err := c.client.Get("/export/do-work")
	if err != nil {
		return fmt.Errorf("failed to GET /export/do-work: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return fmt.Errorf("failed to GET /export/do-work: %w: %s", err, body)
	}
	return nil
}

func checkResp(r *http.Response) ([]byte, error) {
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if r.StatusCode != 200 {
		return nil, fmt.Errorf("response was not 200 OK: %s", body)
	}

	return body, nil
}
