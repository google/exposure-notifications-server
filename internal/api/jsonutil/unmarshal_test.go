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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"

	"github.com/google/go-cmp/cmp"
)

func TestInvalidHeader(t *testing.T) {
	body := ioutil.NopCloser(bytes.NewReader([]byte("")))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("content-type", "application/text")

	w := httptest.NewRecorder()
	data := &model.Publish{}
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
	invalidJSON := []string{
		``,
	}
	errors := []string{
		`body must not be empty`,
	}
	unmarshalTestHelper(t, invalidJSON, errors, http.StatusBadRequest)
}
func TestMultipleJson(t *testing.T) {
	invalidJSON := []string{
		`{"temporaryExposureKeys":
			[{"key": "ABC"},
			 {"key": "DEF"},
			 {"key": "123"}],
		"appPackageName": "com.google.android.awesome",
		"regions": ["us"],
		"deviceVerificationPayload": "foo"}
		{"temporaryExposureKeys":
			[{"key": "ABC"},
			 {"key": "DEF"},
			 {"key": "123"}],
		"appPackageName": "com.google.android.awesome",
		"regions": ["us"],
		"deviceVerificationPayload": "foo"}`,
	}
	errors := []string{
		"body must contain only one JSON object",
	}
	unmarshalTestHelper(t, invalidJSON, errors, http.StatusBadRequest)
}

func TestInvalidJSON(t *testing.T) {
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
	intervalNumber := int32(time.Date(2020, 04, 17, 20, 04, 01, 1, time.UTC).Unix() / 600)
	json := `{"temporaryExposureKeys": [
		  {"key": "ABC", "rollingStartNumber": %v, "rollingPeriod": 144},
		  {"key": "DEF", "rollingStartNumber": %v, "rollingPeriod": 122},
			{"key": "123", "rollingStartNumber": %v, "rollingPeriod": 1}],
    "appPackageName": "com.google.android.awesome",
    "platform": "android",
    "regions": ["CA", "US"],
		"transmissionRisk": 2,
    "DeviceVerificationPayload": "foo",
    "VerificationPayload": "1234-ABCD-EFGH-5678"}`
	json = fmt.Sprintf(json, intervalNumber, intervalNumber, intervalNumber)

	body := ioutil.NopCloser(bytes.NewReader([]byte(json)))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("content-type", "application/json")

	w := httptest.NewRecorder()

	got := &model.Publish{}
	code, err := Unmarshal(w, r, got)
	if err != nil {
		t.Fatalf("unexpected err, %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("unmarshal wanted %v response code, got %v", http.StatusOK, code)
	}

	want := &model.Publish{
		Keys: []model.ExposureKey{
			{Key: "ABC", IntervalNumber: intervalNumber, IntervalCount: 144},
			{Key: "DEF", IntervalNumber: intervalNumber, IntervalCount: 122},
			{Key: "123", IntervalNumber: intervalNumber, IntervalCount: 1},
		},
		Regions:                   []string{"CA", "US"},
		Platform:                  "android",
		AppPackageName:            "com.google.android.awesome",
		TransmissionRisk:          2,
		DeviceVerificationPayload: "foo",
		VerificationPayload:       "1234-ABCD-EFGH-5678",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
	}
}

func unmarshalTestHelper(t *testing.T, payloads []string, errors []string, expCode int) {
	for i, testStr := range payloads {
		body := ioutil.NopCloser(bytes.NewReader([]byte(testStr)))
		r := httptest.NewRequest("POST", "/", body)
		r.Header.Set("content-type", "application/json")

		w := httptest.NewRecorder()
		data := &model.Publish{}
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
