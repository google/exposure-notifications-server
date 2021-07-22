// Copyright 2020 the Exposure Notifications Server authors
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

package generate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/sethvargo/go-envconfig"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func testServer(tb testing.TB) *Server {
	tb.Helper()

	ctx := project.TestContext(tb)
	_, dbConfig := testDatabaseInstance.NewDatabase(tb)

	cfg := &Config{
		Database: *dbConfig,
		SecretManager: secrets.Config{
			Type: "IN_MEMORY",
		},
		ObservabilityExporter: observability.Config{
			ExporterType: observability.ExporterNoop,
		},

		NumExposures:                 5,
		KeysPerExposure:              2,
		MaxKeysOnPublish:             10,
		MaxSameStartIntervalKeys:     2,
		MaxIntervalAge:               360 * time.Hour,
		MaxMagnitudeSymptomOnsetDays: 21,
		CreatedAtTruncateWindow:      1 * time.Hour,
		DefaultRegion:                "US",
		ChanceOfKeyRevision:          0,
		KeyRevisionDelay:             1 * time.Hour,
	}

	env, err := setup.SetupWith(ctx, cfg, envconfig.MapLookuper(map[string]string{
		// We want this to be 0, but envconfig overrides the default value of 0 when
		// processing, so force it to be 0 by simulating the environment variable
		// value to 0.
		"CHANCE_OF_KEY_REVISION": "0",
	}))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := env.Close(ctx); err != nil {
			tb.Fatal(err)
		}
	})

	srv, err := NewServer(cfg, env)
	if err != nil {
		tb.Fatal(err)
	}
	return srv
}

func TestServer_smoke(t *testing.T) {
	// Note: this only tests that the server is capable of sending and receiving
	// responses. It does not attempt to exhaustively exercise the actual
	// generation, which is tested in a different function.
	t.Parallel()

	ctx := project.TestContext(t)

	srv := testServer(t)
	mux := srv.Routes(ctx)

	t.Run("handleGenerate", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest("POST", "/", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)

		w.Flush()

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}

		if got, want := w.Body.String(), `{"ok":true}`; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	})
}

func TestServer_handleGenerate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cases := []struct {
		name string
		err  string

		regions        []string
		sameDayRelease bool
		reviseKeys     bool
	}{
		{
			name: "not_enough_keys",
			err:  "must be at least 2",
		},
		{
			name:           "same_day_release",
			sameDayRelease: true,
			err:            "TODO",
		},
		{
			name:       "revised_keys",
			reviseKeys: true,
			err:        "TODO",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := testServer(t)

			if tc.sameDayRelease {
				srv.config.SimulateSameDayRelease = true
			}
			if tc.reviseKeys {
				srv.config.ChanceOfKeyRevision = 100
			}

			if err := srv.generate(ctx, tc.regions); err != nil {
				if tc.err == "" {
					t.Fatal(err)
				}
				if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
					t.Errorf("expected %#v to contain %#v", got, want)
				}
			}

			var exposures []*publishmodel.Exposure
			if _, err := srv.database.IterateExposures(ctx, publishdb.IterateExposuresCriteria{}, func(m *publishmodel.Exposure) error {
				exposures = append(exposures, m)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			if got, want := len(exposures), 0; got < want {
				t.Errorf("expected at least one exposure")
			}
		})
	}
}
