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

// Package enclient is a client for making requests against the exposure
// notification server.
package enclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

const (
	// httpTimeout is the maximum amount of time to wait for a response.
	httpTimeout = 5 * time.Minute
)

type Interval int32

// PostRequest requests to the specified url. This methods attempts to serialize
// data argument as a json.
func PostRequest(url string, data interface{}) (*http.Response, error) {
	request := bytes.NewBuffer(JSONRequest(data))
	r, err := http.NewRequest("POST", url, request)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(r)
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

// JSONRequest serializes the given argument to json.
func JSONRequest(data interface{}) []byte {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("unable to marshal json payload")
	}
	return jsonData
}

// NowInterval returns the Interval for the current moment of tme.
func NowInterval() Interval {
	return NewInterval(time.Now().Unix())
}

// NewInterval creates a new interval for the UNIX epoch given.
func NewInterval(time int64) Interval {
	return Interval(int32(time / 600))
}

// ExposureKey creates an exposure key.
func ExposureKey(key string, intervalNumber Interval, intervalCount int32, transmissionRisk int) verifyapi.ExposureKey {
	return verifyapi.ExposureKey{
		Key:              key,
		IntervalNumber:   int32(intervalNumber),
		IntervalCount:    intervalCount,
		TransmissionRisk: transmissionRisk,
	}
}
