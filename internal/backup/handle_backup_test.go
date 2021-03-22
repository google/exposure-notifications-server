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

package backup

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

func TestServer_HandleBackup(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	t.Run("bad_database", func(t *testing.T) {
		t.Parallel()

		testDB, _ := testDatabaseInstance.NewDatabase(t)
		env := serverenv.New(ctx, serverenv.WithDatabase(testDB))

		cfg := &Config{
			DatabaseInstanceURL: "https://example.com",
		}
		c, err := NewServer(cfg, env)
		if err != nil {
			t.Fatal(err)
		}
		handler := c.handleBackup()

		r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body)
		}
	})

	t.Run("bad_endpoint", func(t *testing.T) {
		t.Parallel()

		testDB, _ := testDatabaseInstance.NewDatabase(t)
		env := serverenv.New(ctx, serverenv.WithDatabase(testDB))

		cfg := &Config{
			DatabaseInstanceURL: "\x7f",
		}
		c, err := NewServer(cfg, env)
		if err != nil {
			t.Fatal(err)
		}
		handler := c.handleBackup()

		r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body)
		}
		if got, want := w.Body.String(), "failed to parse database instance url"; !strings.Contains(got, want) {
			t.Errorf("expected body to contain %q: %s", want, got)
		}
	})

	t.Run("bad_url", func(t *testing.T) {
		t.Parallel()

		testDB, _ := testDatabaseInstance.NewDatabase(t)
		env := serverenv.New(ctx, serverenv.WithDatabase(testDB))

		cfg := &Config{
			DatabaseInstanceURL: "https://not-a-real.web.site.no",
		}
		c, err := NewServer(cfg, env)
		if err != nil {
			t.Fatal(err)
		}
		handler := c.handleBackup()

		r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body)
		}
		if got, want := w.Body.String(), "failed to execute request"; !strings.Contains(got, want) {
			t.Errorf("expected body to contain %q: %s", want, got)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		testDB, _ := testDatabaseInstance.NewDatabase(t)
		env := serverenv.New(ctx, serverenv.WithDatabase(testDB))

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}))
		defer ts.Close()

		cfg := &Config{
			MinTTL:              15 * time.Second,
			Bucket:              "bucket",
			DatabaseInstanceURL: ts.URL + "/instance",
			DatabaseName:        "name",
		}

		c, err := NewServer(cfg, env)
		if err != nil {
			t.Fatal(err)
		}
		handler := c.handleBackup()

		r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body)
		}
	})

	t.Run("bad_upstream_response", func(t *testing.T) {
		t.Parallel()

		testDB, _ := testDatabaseInstance.NewDatabase(t)
		env := serverenv.New(ctx, serverenv.WithDatabase(testDB))

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			fmt.Fprint(w, "badness")
		}))
		defer ts.Close()

		cfg := &Config{
			Bucket:              "bucket",
			DatabaseInstanceURL: ts.URL + "/instance",
			DatabaseName:        "name",
		}

		c, err := NewServer(cfg, env)
		if err != nil {
			t.Fatal(err)
		}
		handler := c.handleBackup()

		r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body)
		}
		if got, want := w.Body.String(), "unsuccessful response from backup"; !strings.Contains(got, want) {
			t.Errorf("expected body to contain %q: %s", want, got)
		}
	})
}
