// +build e2e

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
// See the License for the specific la

package e2etest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/integration"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/sethvargo/go-envconfig"

	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	testutil "github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

type testConfig struct {
	DbName       string `env:"DB_NAME"`
	ExposureURL  string `env:"EXPOSURE_URL"`
	ExportBucket string `env:"EXPORT_BUCKET"`
	ProjectID    string `env:"PROJECT_ID"`
	DBConfig     *database.Config
}

func initConfig(tb testing.TB, ctx context.Context) *testConfig {
	c := &testConfig{}
	sm, err := secrets.SecretManagerFor(ctx, secrets.SecretManagerTypeGoogleSecretManager)
	if err != nil {
		tb.Fatalf("unable to connect to secret manager: %v", err)
	}
	if err := envconfig.ProcessWith(ctx, c, envconfig.OsLookuper(), secrets.Resolver(sm, &secrets.Config{})); err != nil {
		tb.Fatalf("Unable to process environment: %v", err)
	}
	return c
}

func publishKeys(payload *verifyapi.Publish, publishEndpoint string) (*verifyapi.PublishResponse, error) {
	j, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	resp, err := http.Post(publishEndpoint, "application/json", bytes.NewReader(j))
	if err != nil {
		return nil, fmt.Errorf("failed to POST /publish: %w", err)
	}

	body, err := checkResp(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to POST /publish: %w: %s", err, body)
	}

	var pubResponse verifyapi.PublishResponse
	if err := json.Unmarshal(body, &pubResponse); err != nil {
		return nil, fmt.Errorf("bad publish response")
	}

	return &pubResponse, nil
}

func checkResp(r *http.Response) ([]byte, error) {
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if r.StatusCode != 200 {
		return nil, fmt.Errorf("response was not 200 OK: %s", body)
	}

	return body, nil
}

func TestPublishEndpoint(t *testing.T) {
	ctx := context.Background()

	tc := initConfig(t, ctx)
	if tc.ExposureURL == "" {
		t.Skip()
	}

	db, err := database.NewFromEnv(ctx, tc.DBConfig)
	if err != nil {
		t.Fatalf("unable to connect to database: %v", err)
	}
	jwtCfg, _, _ := integration.Seed(t, ctx, db, 2*time.Second)
	keys := util.GenerateExposureKeys(3, -1, false)

	// Publish 3 keys
	payload := &verifyapi.Publish{
		Keys:              keys,
		HealthAuthorityID: "com.example.app",
	}
	jwtCfg.ExposureKeys = keys
	jwtCfg.JWTWarp = time.Duration(0)
	verification, salt := testutil.IssueJWT(t, jwtCfg)
	payload.VerificationPayload = verification
	payload.HMACKey = salt
	resp, err := publishKeys(payload, tc.ExposureURL+"/v1/publish")
	if err != nil {
		t.Fatalf("Failed publishing keys: \n\tResp: %v\n\t%v", resp, err)
	}

	criteria := publishdb.IterateExposuresCriteria{
		OnlyLocalProvenance: false,
	}
	exposures, err := getExposures(db, criteria)
	if err != nil {
		t.Fatalf("Failed getting exposures: %v", err)
	}

	keysPublished := make(map[string]bool)
	for _, e := range exposures {
		strKey := base64.StdEncoding.EncodeToString(e.ExposureKey)
		keysPublished[strKey] = true
	}

	for _, want := range keys {
		if _, ok := keysPublished[want.Key]; !ok {
			t.Fatalf("Want published key %q not exist in exposures", want.Key)
		}
	}
}

// getExposures finds the exposures that match the given criteria.
func getExposures(db *database.DB, criteria publishdb.IterateExposuresCriteria) ([]*publishmodel.Exposure, error) {
	ctx := context.Background()
	var exposures []*publishmodel.Exposure
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		exposures = append(exposures, m)
		return nil
	}); err != nil {
		return nil, err
	}

	return exposures, nil
}
