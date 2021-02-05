// Copyright 2021 Google LLC
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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type testConfig struct {
	enabled bool
}

func (t *testConfig) MaintenanceMode() bool {
	return t.enabled
}

func TestHandle_Enabled(t *testing.T) {
	responder := ProcessMaintenance(&testConfig{true})

	r := &http.Request{}
	w := httptest.NewRecorder()

	responder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler was invoked")
	})).ServeHTTP(w, r)

	w.Flush()

	if got, want := w.Code, 429; got != want {
		t.Errorf("expected %d to be %d", got, want)
	}
}

func TestHandle_Disabled(t *testing.T) {
	responder := ProcessMaintenance(&testConfig{false})

	r := &http.Request{}
	w := httptest.NewRecorder()

	responder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})).ServeHTTP(w, r)

	w.Flush()

	if got, want := w.Code, 200; got != want {
		t.Errorf("expected %d to be %d", got, want)
	}
}
