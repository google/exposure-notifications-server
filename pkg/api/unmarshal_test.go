package api

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cambio/pkg/model"

	"github.com/google/go-cmp/cmp"
)

func TestInvalidHeader(t *testing.T) {
	body := ioutil.NopCloser(bytes.NewReader([]byte("")))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("content-type", "application/text")

	w := httptest.NewRecorder()
	data := &model.Publish{}
	err, code := unmarshal(w, r, data)

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
		`{"diagnosisKeys":
			[{"key": "ABC"},
			 {"key": "DEF"},
			 {"key": "123"}],
		"appPackageName": "com.google.android.awesome",
		"regions": ["us"],
		"verificationPayload": "foo"}
		{"diagnosisKeys":
			[{"key": "ABC"},
			 {"key": "DEF"},
			 {"key": "123"}],
		"appPackageName": "com.google.android.awesome",
		"regions": ["us"],
		"verificationPayload": "foo"}`,
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
		`{"diagnosisKeys": ["ABC", "DEF", "123"],`,
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
		`{"diagnosisKeys": 42}`,
		`{"diagnosisKeys": ["41", 42]}`,
		`{"appPackageName": 4.5}`,
		`{"regions": "us"}`,
		`{"badField": "doesn't exist"}`,
	}
	errors := []string{
		`invalid value diagnosisKeys at position 20`,
		`invalid value diagnosisKeys at position 23`,
		`invalid value appPackageName at position 22`,
		`invalid value regions at position 16`,
		`unknown field "badField"`,
	}
	unmarshalTestHelper(t, invalidJSON, errors, http.StatusBadRequest)
}

func TestValidPublisMessage(t *testing.T) {
	keyDay := time.Date(2020, 04, 17, 20, 04, 01, 1, time.UTC).Unix()
	json := `{"diagnosisKeys": [
		  {"key": "ABC", "keyDay": %v},
		  {"key": "DEF", "keyDay": %v},
			{"key": "123", "keyDay": %v}],
    "appPackageName": "com.google.android.awesome",
    "regions": ["CA", "US"],
		"diagnosisStatus": 2,
    "verificationPayload": "foo"}`
	json = fmt.Sprintf(json, keyDay, keyDay, keyDay)

	body := ioutil.NopCloser(bytes.NewReader([]byte(json)))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("content-type", "application/json")

	w := httptest.NewRecorder()

	got := &model.Publish{}
	err, code := unmarshal(w, r, got)
	if err != nil {
		t.Fatalf("unexpected err, %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("unmarshal wanted %v response code, got %v", http.StatusOK, code)
	}

	want := &model.Publish{
		Keys: []model.DiagnosisKey{
			model.DiagnosisKey{Key: "ABC", KeyDay: keyDay},
			model.DiagnosisKey{Key: "DEF", KeyDay: keyDay},
			model.DiagnosisKey{Key: "123", KeyDay: keyDay},
		},
		Regions:         []string{"CA", "US"},
		AppPackageName:  "com.google.android.awesome",
		DiagnosisStatus: 2,
		Verification:    "foo",
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
		err, code := unmarshal(w, r, data)
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
