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

package generate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/setup"
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

func testHandler(tb testing.TB) (*Config, *serverenv.ServerEnv) {
	tb.Helper()

	ctx := context.Background()
	_, dbConfig := testDatabaseInstance.NewDatabase(tb)

	config := &Config{
		Database: *dbConfig,
		SecretManager: secrets.Config{
			SecretManagerType: secrets.SecretManagerTypeInMemory,
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

	env, err := setup.SetupWith(ctx, config, envconfig.MapLookuper(map[string]string{
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

	return config, env
}

func TestGenerateHandler_ServeHTTP(t *testing.T) {
	// Note: this only tests that the server is capable of sending and receiving
	// responses. It does not attempt to exhaustively exercise the actual
	// generation, which is tested in a different function.
	t.Parallel()

	ctx := context.Background()
	config, env := testHandler(t)
	h, err := NewHandler(ctx, config, env)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(h.ServeHTTP)
	handler.ServeHTTP(rr, req)

	if got, want := rr.Code, http.StatusOK; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}

	if got, want := rr.Body.String(), "successfully generated exposure keys"; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}
}

func TestGenerateHandler_Generate(t *testing.T) {
	t.Parallel()

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

			config, env := testHandler(t)
			db := env.Database()

			if tc.sameDayRelease {
				config.SimulateSameDayRelease = true
			}
			if tc.reviseKeys {
				config.ChanceOfKeyRevision = 100
			}

			ctx := context.Background()
			h, err := NewHandler(ctx, config, env)
			if err != nil {
				t.Fatal(err)
			}
			gh, ok := h.(*generateHandler)
			if !ok {
				t.Fatalf("expected handler to be generateHandler")
			}

			if err := gh.generate(ctx, tc.regions); err != nil {
				if tc.err == "" {
					t.Fatal(err)
				}
				if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
					t.Errorf("expected %#v to contain %#v", got, want)
				}
			}

			var exposures []*publishmodel.Exposure
			if _, err := publishdb.New(db).IterateExposures(ctx, publishdb.IterateExposuresCriteria{}, func(m *publishmodel.Exposure) error {
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
