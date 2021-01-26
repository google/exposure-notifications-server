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

// Package maintenance provides utilities for maintenance mode handling
package maintenance

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

func testHandler(invoked *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*invoked = true
	})
}

func TestHandle_Enabled(t *testing.T) {
	responder := New(&testConfig{true})

	r := &http.Request{}
	w := httptest.NewRecorder()

	invoked := false
	responder.Handle(testHandler(&invoked)).ServeHTTP(w, r)

	if invoked {
		t.Fatalf("handler was invoked while in maintenance mode")
	}
}

func TestHandle_Disabled(t *testing.T) {
	responder := New(&testConfig{false})

	r := &http.Request{}
	w := httptest.NewRecorder()

	invoked := false
	responder.Handle(testHandler(&invoked)).ServeHTTP(w, r)

	if !invoked {
		t.Fatalf("handler was not invoked while not in maintenance mode")
	}
}
