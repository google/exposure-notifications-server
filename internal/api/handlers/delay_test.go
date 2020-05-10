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

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type th struct{}

func (*th) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	return
}

func TestDelayedReturn(t *testing.T) {
	cases := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		750 * time.Millisecond,
		1250 * time.Millisecond,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler := &th{}

	for i, c := range cases {
		handler := WithMinimumLatency(c, handler)
		st := time.Now()
		handler(w, r)
		et := time.Now()
		if et.Sub(st) < c {
			t.Errorf("%v latency too low, got %v, want > %v", i, et.Sub(st), c)
		}
	}
}
