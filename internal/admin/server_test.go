// Copyright 2021 the Exposure Notifications Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-server/internal/project"
)

func TestServerRoutes(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	_, s := newTestServer(t)
	h := s.Routes(ctx)

	cases := []struct {
		name string
		path string
	}{
		{"index", "/"},
		{"apps", "/app"},
		{"health_authority", "/healthauthority/0"},
		{"exports", "/exports/0"},
		{"export_importers", "/export-importers/0"},
		{"mirrors", "/mirrors/0"},
		{"siginfo", "/siginfo/0"},
		{"health", "/health"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, err := http.NewRequestWithContext(ctx, http.MethodGet, tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			w.Flush()

			if got, want := w.Code, 200; got != want {
				t.Errorf("expected status %d to be %d; headers: %#v; body: %s", got, want, w.Header(), w.Body.String())
			}
		})
	}
}
