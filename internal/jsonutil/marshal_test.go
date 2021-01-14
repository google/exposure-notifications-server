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

package jsonutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEscapeJSON(t *testing.T) {
	want := `{\"a\": \"b\"}`
	got := escapeJSON(`{"a": "b"}`)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
	}
}

func TestMarshalResponse(t *testing.T) {
	w := httptest.NewRecorder()

	toSave := map[string]string{
		"name": "Steve",
	}

	MarshalResponse(w, http.StatusOK, toSave)

	if w.Code != http.StatusOK {
		t.Errorf("wrong response code, want: %v got: %v", http.StatusOK, w.Code)
	}

	got := w.Body.String()
	want := `{"name":"Steve"}`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
	}
}

func TestMarshalResponseError(t *testing.T) {
	w := httptest.NewRecorder()

	type Circular struct {
		Name string    `json:"name"`
		Next *Circular `json:"next"`
	}

	badInput := &Circular{
		Name: "Bob",
	}
	badInput.Next = badInput

	MarshalResponse(w, http.StatusInternalServerError, badInput)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("wrong response code, want: %v got: %v", http.StatusOK, w.Code)
	}

	got := w.Body.String()
	want := `{"error":"json: unsupported value: encountered a cycle via *jsonutil.Circular"}`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unmarshal mismatch (-want +got):\n%v", diff)
	}
}
