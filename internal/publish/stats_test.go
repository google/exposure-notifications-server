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

package publish

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	pubdb "github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	testutil "github.com/google/exposure-notifications-server/internal/utils"
	vermodel "github.com/google/exposure-notifications-server/internal/verification/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sethvargo/go-envconfig"
)

func TestRetrieveMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	authKey := testutil.GetSigningKey(t)
	kms := keys.TestKeyManager(t)
	keyID := keys.TestEncryptionKey(t, kms)

	tokenAAD := make([]byte, 16)
	if _, err := rand.Read(tokenAAD); err != nil {
		t.Fatalf("not enough entropy: %v", err)
	}

	startTime := timeutils.UTCMidnight(time.Now().UTC()).Add(-48 * time.Hour)

	// load default config to ensure that what we need is there.
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		t.Fatal(err)
	}
	aaProvider, err := authorizedapp.NewDatabaseProvider(ctx, testDB, config.AuthorizedAppConfig())
	if err != nil {
		t.Fatal(err)
	}
	config.RevisionToken.AAD = tokenAAD
	config.RevisionToken.KeyID = keyID
	env := serverenv.New(ctx,
		serverenv.WithDatabase(testDB),
		serverenv.WithAuthorizedAppProvider(aaProvider),
		serverenv.WithKeyManager(kms))

	// Create a health authority with a public key.
	healthAuthority := &vermodel.HealthAuthority{
		Issuer:         "health-authority",
		Audience:       "n/a",
		Name:           "health-authority",
		EnableStatsAPI: true,
	}
	healthAuthorityKey := &vermodel.HealthAuthorityKey{
		Version: "v1",
		From:    time.Now().Add(-1 * time.Minute),
	}
	healthAuthorityID := testutil.InitializeVerificationDB(ctx, t, testDB, healthAuthority, healthAuthorityKey, authKey)

	// Add some raw publish info metrics.
	rawStats := []model.PublishInfo{
		{
			CreatedAt:    startTime,
			Platform:     model.PlatformAndroid,
			NumTEKs:      14,
			OldestDays:   14,
			OnsetDaysAgo: 4,
		},
		{
			CreatedAt:    startTime,
			Platform:     model.PlatformIOS,
			NumTEKs:      10,
			OldestDays:   10,
			OnsetDaysAgo: 3,
		},
	}

	pubDB := pubdb.New(testDB)
	// These are stacked to ensure that we have enough data to come back out (and some that won't)
	for days := 0; days <= 2; days++ {
		for hours := 0; hours < 10; hours++ {
			if days == 2 && hours == 1 {
				break
			}
			for _, template := range rawStats {
				info := template
				info.CreatedAt = template.CreatedAt.Add(time.Duration(days*24+hours) * time.Hour)
				if err := pubDB.UpdateStats(ctx, info.CreatedAt.Truncate(time.Hour), healthAuthorityID, &info); err != nil {
					t.Fatalf("unable to update stats: %v", err)
				}
			}
		}
	}

	pubHandler, err := NewHandler(ctx, &config, env)
	if err != nil {
		t.Fatalf("unable to create publish handler: %v", err)
	}
	metricsHandler := pubHandler.HandleStats()

	// get the authentication token.
	jwtConfig := &testutil.StatsJWTConfig{
		HealthAuthority:    healthAuthority,
		HealthAuthorityKey: healthAuthorityKey,
		Key:                authKey.Key,
		Audience:           config.Verification.StatsAudience,
	}
	token := jwtConfig.IssueStatsJWT(t)

	// make the stats request.
	request := &verifyapi.StatsRequest{}
	jsonString, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	httpRequest, err := http.NewRequest("POST", "", strings.NewReader(string(jsonString)))
	if err != nil {
		t.Fatal(err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	rr := httptest.NewRecorder()
	metricsHandler.ServeHTTP(rr, httpRequest)
	resp := rr.Result()

	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var got verifyapi.StatsResponse
	if err := json.Unmarshal(respBytes, &got); err != nil {
		t.Fatalf("unable to unmarshal response body: %v; data: %v", err, string(respBytes))
	}

	want := verifyapi.StatsResponse{
		Days: []*verifyapi.StatsDay{
			{
				Day: startTime,
				PublishRequests: verifyapi.PublishRequests{
					Android: 10,
					IOS:     10,
				},
				TotalTEKsPublished:        240,
				TEKAgeDistribution:        []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 0, 10, 0},
				OnsetToUploadDistribution: []int64{0, 0, 0, 10, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			},
			{
				Day: startTime.Add(24 * time.Hour),
				PublishRequests: verifyapi.PublishRequests{
					Android: 10,
					IOS:     10,
				},
				TotalTEKsPublished:        240,
				TEKAgeDistribution:        []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 0, 10, 0},
				OnsetToUploadDistribution: []int64{0, 0, 0, 10, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			},
		},
	}

	ignorePadding := cmpopts.IgnoreFields(verifyapi.StatsResponse{}, "Padding")
	if diff := cmp.Diff(want, got, ignorePadding); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
	if got.Padding == "" {
		t.Errorf("response is missing padding")
	}
}

func TestRetrieveMetrics_AuthErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	authKey := testutil.GetSigningKey(t)
	kms := keys.TestKeyManager(t)
	keyID := keys.TestEncryptionKey(t, kms)

	tokenAAD := make([]byte, 16)
	if _, err := rand.Read(tokenAAD); err != nil {
		t.Fatalf("not enough entropy: %v", err)
	}

	// load default config to ensure that what we need is there.
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		t.Fatal(err)
	}
	aaProvider, err := authorizedapp.NewDatabaseProvider(ctx, testDB, config.AuthorizedAppConfig())
	if err != nil {
		t.Fatal(err)
	}
	config.RevisionToken.AAD = tokenAAD
	config.RevisionToken.KeyID = keyID
	env := serverenv.New(ctx,
		serverenv.WithDatabase(testDB),
		serverenv.WithAuthorizedAppProvider(aaProvider),
		serverenv.WithKeyManager(kms))

	// Create a health authority with a public key.
	healthAuthority := &vermodel.HealthAuthority{
		Issuer:         "health-authority",
		Audience:       "n/a",
		Name:           "health-authority",
		EnableStatsAPI: true,
	}
	healthAuthorityKey := &vermodel.HealthAuthorityKey{
		Version: "v1",
		From:    time.Now().Add(-1 * time.Minute),
	}
	_ = testutil.InitializeVerificationDB(ctx, t, testDB, healthAuthority, healthAuthorityKey, authKey)

	pubHandler, err := NewHandler(ctx, &config, env)
	if err != nil {
		t.Fatalf("unable to create publish handler: %v", err)
	}
	metricsHandler := pubHandler.HandleStats()

	// get the authentication token.
	jwtConfig := &testutil.StatsJWTConfig{
		HealthAuthority:    healthAuthority,
		HealthAuthorityKey: healthAuthorityKey,
		Key:                authKey.Key,
		Audience:           config.Verification.StatsAudience + "WRONG",
	}
	token := jwtConfig.IssueStatsJWT(t)

	cases := []struct {
		Name          string
		Authorization string
		ErrorMessage  string
		ErrorCode     string
	}{
		{
			Name:          "bad_token",
			Authorization: fmt.Sprintf("Bearer %s", token),
			ErrorMessage:  "unauthorized, audience mismatch",
			ErrorCode:     "unauthorized",
		},
		{
			Name:          "not_bearer",
			Authorization: "LET ME IN",
			ErrorMessage:  "Authorization header is not in `Bearer <token>` format",
			ErrorCode:     "unauthorized",
		},
		{
			Name:         "missing_auth_header",
			ErrorMessage: "Authorization header is not in `Bearer <token>` format",
			ErrorCode:    "unauthorized",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			// make the stats request with auth token.
			request := &verifyapi.StatsRequest{}
			jsonString, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			httpRequest, err := http.NewRequest("POST", "", strings.NewReader(string(jsonString)))
			if err != nil {
				t.Fatal(err)
			}
			httpRequest.Header.Set("Content-Type", "application/json")
			if tc.Authorization != "" {
				httpRequest.Header.Set("Authorization", tc.Authorization)
			}
			rr := httptest.NewRecorder()
			metricsHandler.ServeHTTP(rr, httpRequest)
			resp := rr.Result()

			defer resp.Body.Close()
			respBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			var got verifyapi.StatsResponse
			if err := json.Unmarshal(respBytes, &got); err != nil {
				t.Fatalf("unable to unmarshal response body: %v; data: %v", err, string(respBytes))
			}

			want := verifyapi.StatsResponse{
				ErrorMessage: tc.ErrorMessage,
				ErrorCode:    tc.ErrorCode,
			}

			ignorePadding := cmpopts.IgnoreFields(verifyapi.StatsResponse{}, "Padding")
			if diff := cmp.Diff(want, got, ignorePadding); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
			if got.Padding == "" {
				t.Errorf("response is missing padding")
			}
		})
	}
}
