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

package jsonutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"

	"github.com/google/go-cmp/cmp"
)

func TestBodyTooLarge(t *testing.T) {
	t.Parallel()
	input := make(map[string]string, 1)
	input["padding"] = strings.Repeat("0", maxBodyBytes+10)

	largeJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	errors := []string{
		`http: request body too large`,
	}
	unmarshalTestHelper(t, []string{string(largeJSON)}, errors, http.StatusRequestEntityTooLarge)
}

func TestInvalidHeader(t *testing.T) {
	t.Parallel()
	body := ioutil.NopCloser(bytes.NewReader([]byte("")))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("content-type", "application/text")

	w := httptest.NewRecorder()
	data := &verifyapi.Publish{}
	code, err := Unmarshal(w, r, data)

	expCode := http.StatusUnsupportedMediaType
	expErr := "content-type is not application/json"
	if code != expCode {
		t.Errorf("unmarshal wanted %v response code, got %v", expCode, code)
	}

	if err == nil || err.Error() != expErr {
		t.Errorf("expected error '%v', got: %v", expErr, err)
	}
}

func TestEmptyBody(t *testing.T) {
	t.Parallel()
	invalidJSON := []string{
		``,
	}
	errors := []string{
		`body must not be empty`,
	}
	unmarshalTestHelper(t, invalidJSON, errors, http.StatusBadRequest)
}
func TestMultipleJson(t *testing.T) {
	t.Parallel()
	invalidJSON := []string{
		`{"temporaryExposureKeys":
			[{"key": "ABC"},
			 {"key": "DEF"},
			 {"key": "123"}],
		"appPackageName": "com.google.android.awesome",
		"regions": ["us"]}
		{"temporaryExposureKeys":
			[{"key": "ABC"},
			 {"key": "DEF"},
			 {"key": "123"}],
		"appPackageName": "com.google.android.awesome",
		"regions": ["us"]}`,
	}
	errors := []string{
		"body must contain only one JSON object",
	}
	unmarshalTestHelper(t, invalidJSON, errors, http.StatusBadRequest)
}

func TestInvalidJSON(t *testing.T) {
	t.Parallel()
	invalidJSON := []string{
		`totally not json`,
		`{"key": "value", badKey: 6`,
		`{"exposureKeys": ["ABC", "DEF", "123"],`,
	}
	errors := []string{
		`malformed json at position 2`,
		`malformed json at position 18`,
		`malformed json`,
	}
	unmarshalTestHelper(t, invalidJSON, errors, http.StatusBadRequest)
}

func TestInvalidStructure(t *testing.T) {
	t.Parallel()
	invalidJSON := []string{
		`{"temporaryExposureKeys": 42}`,
		`{"temporaryExposureKeys": ["41", 42]}`,
		`{"appPackageName": 4.5}`,
		`{"regions": "us"}`,
		`{"badField": "doesn't exist"}`,
	}
	errors := []string{
		`invalid value temporaryExposureKeys at position 28`,
		`invalid value temporaryExposureKeys at position 31`,
		`invalid value appPackageName at position 22`,
		`invalid value regions at position 16`,
		`unknown field "badField"`,
	}
	unmarshalTestHelper(t, invalidJSON, errors, http.StatusBadRequest)
}

func TestValidPublishMessage(t *testing.T) {
	t.Parallel()
	intervalNumber := int32(time.Date(2020, 04, 17, 20, 04, 01, 1, time.UTC).Unix() / 600)
	json := `{"temporaryExposureKeys": [
		  {"key": "ABC", "rollingStartNumber": %v, "rollingPeriod": 144, "TransmissionRisk": 2},
		  {"key": "DEF", "rollingStartNumber": %v, "rollingPeriod": 122, "TransmissionRisk": 2},
			{"key": "123", "rollingStartNumber": %v, "rollingPeriod": 1, "TransmissionRisk": 2}],
    "appPackageName": "com.google.android.awesome",
    "regions": ["CA", "US"],
    "VerificationPayload": "1234-ABCD-EFGH-5678"}`
	json = fmt.Sprintf(json, intervalNumber, intervalNumber, intervalNumber)

	body := ioutil.NopCloser(bytes.NewReader([]byte(json)))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("content-type", "application/json")

	w := httptest.NewRecorder()

	got := &verifyapi.Publish{}
	code, err := Unmarshal(w, r, got)
	if err != nil {
		t.Fatalf("unexpected err, %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("unmarshal wanted %v response code, got %v", http.StatusOK, code)
	}

	want := &verifyapi.Publish{
		Keys: []verifyapi.ExposureKey{
			{Key: "ABC", IntervalNumber: intervalNumber, IntervalCount: 144, TransmissionRisk: 2},
			{Key: "DEF", IntervalNumber: intervalNumber, IntervalCount: 122, TransmissionRisk: 2},
			{Key: "123", IntervalNumber: intervalNumber, IntervalCount: 1, TransmissionRisk: 2},
		},
		Regions:             []string{"CA", "US"},
		AppPackageName:      "com.google.android.awesome",
		VerificationPayload: "1234-ABCD-EFGH-5678",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
	}
}

func unmarshalTestHelper(t *testing.T, payloads []string, errors []string, expCode int) {
	t.Helper()
	for i, testStr := range payloads {
		body := ioutil.NopCloser(bytes.NewReader([]byte(testStr)))
		r := httptest.NewRequest("POST", "/", body)
		r.Header.Set("content-type", "application/json; charset=utf-8")

		w := httptest.NewRecorder()
		data := &verifyapi.Publish{}
		code, err := Unmarshal(w, r, data)
		if code != expCode {
			t.Errorf("unmarshal wanted %v response code, got %v", expCode, code)
		}
		if errors[i] == "" {
			// No error expected for this test, bad if we got one.
			if err != nil {
				t.Errorf("expected no error for `%v`, got: %v", testStr, err)
			}
		} else {
			if err == nil {
				t.Errorf("wanted error '%v', got nil", errors[i])
			} else if err.Error() != errors[i] {
				t.Errorf("expected error '%v', got: %v", errors[i], err)
			}
		}
	}
}
