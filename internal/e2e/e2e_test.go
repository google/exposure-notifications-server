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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	exportapi "github.com/google/exposure-notifications-server/internal/export"
	exportdb "github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/integration"
	"github.com/google/exposure-notifications-server/internal/project"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/go-cmp/cmp"
	pgx "github.com/jackc/pgx/v4"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-retry"

	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	testutil "github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

type testConfig struct {
	Database *database.Config `env:",prefix=E2E_"`

	ExposureURL string `env:"E2E_EXPOSURE_URL"`
	ProjectID   string `env:"E2E_PROJECT_ID"`
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
	t.Parallel()

	ctx := project.TestContext(t)

	tc := initConfig(t, ctx)
	if tc.ExposureURL == "" {
		t.Skip()
	}
	// Increase this so that the db connection won't be canceled while polling for exported files
	tc.Database.PoolMaxConnIdle = 10 * time.Minute

	db, err := database.NewFromEnv(ctx, tc.Database)
	if err != nil {
		t.Fatalf("unable to connect to database: %v", err)
	}
	jwtCfg, bucketName, filenameRoot, appName := integration.Seed(t, ctx, db, 2*time.Minute)

	keys := util.GenerateExposureKeys(3, -1, false)

	// Publish 3 keys
	payload := &verifyapi.Publish{
		Keys:              keys,
		HealthAuthorityID: appName,
	}

	wantExport := make(map[string]bool)
	for _, key := range keys {
		wantExport[key.Key] = true
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
	exposures, err := getExposures(ctx, db, criteria)
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

	blobStore, err := storage.NewGoogleCloudStorage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotExported := make(map[string]bool)
	var expectedFile string
	integration.Eventually(t, 30, 10*time.Second, func() error {
		// Attempt to get the index
		index, err := blobStore.GetObject(ctx, bucketName, integration.IndexFilePath(filenameRoot))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return retry.RetryableError(fmt.Errorf("Can not find index file: %w", err))
			}
			return err
		}

		// Find the latest file in the index
		lines := strings.Split(string(index), "\n")
		latest := ""
		for _, entry := range lines {
			if strings.HasSuffix(entry, "zip") {
				if entry > latest {
					latest = entry
				}
			}
		}
		if latest == "" {
			return retry.RetryableError(errors.New("failed to find latest export"))
		}

		// Download the latest export file contents
		data, err := blobStore.GetObject(ctx, bucketName, latest)
		if err != nil {
			return fmt.Errorf("failed to open %s/%s: %w", bucketName, latest, err)
		}

		// Process contents as an export
		key, _, err := exportapi.UnmarshalExportFile(data)
		if err != nil {
			return fmt.Errorf("failed to extract export data: %w", err)
		}

		for _, k := range key.Keys {
			sk := base64.StdEncoding.EncodeToString(k.KeyData)
			if _, ok := wantExport[sk]; ok {
				expectedFile = latest
				gotExported[sk] = true
			}
		}
		if diff := cmp.Diff(wantExport, gotExported); diff != "" {
			return retry.RetryableError(errors.New("Not all keys are exported yet, keep waiting"))
		}

		return nil
	})

	// Mark the export in the past to force a cleanup
	t.Logf("Exposure was exported: %q. Modify the date to 14 days earlier", expectedFile)
	exportFile, err := exportdb.New(db).LookupExportFile(ctx, expectedFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
		UPDATE
			ExportBatch
		SET
			start_timestamp = $1,
			end_timestamp = $2
		WHERE
			batch_id = $3
	`,
			time.Now().Add(-30*24*time.Hour),
			time.Now().Add(-29*24*time.Hour),
			exportFile.BatchID,
		)
		if err != nil {
			return err
		}
		if got, want := result.RowsAffected(), int64(1); got != want {
			return fmt.Errorf("expected %v to be %v", got, want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Ensure the export was deleted
	// Cloud scheduler runs every minute, waiting 2.5 minutes should be sufficient
	t.Logf("Wait until export %q is cleaned up", expectedFile)
	integration.Eventually(t, 30, 10*time.Second, func() error {
		// Attempt to get the index
		index, err := blobStore.GetObject(ctx, bucketName, integration.IndexFilePath(filenameRoot))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return retry.RetryableError(err)
			}
			return err
		}

		// Ensure the new export is created
		batchFiles := strings.Split(string(index), "\n")
		for _, f := range batchFiles {
			if f != exportFile.Filename {
				continue
			}

			// Lookup the file, hope it's gone
			if _, err := blobStore.GetObject(ctx, bucketName, f); err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return nil // expected
				}
				return err
			}

			return retry.RetryableError(fmt.Errorf("export file still exists"))
		}

		return nil
	})
}

// getExposures finds the exposures that match the given criteria.
func getExposures(ctx context.Context, db *database.DB, criteria publishdb.IterateExposuresCriteria) ([]*publishmodel.Exposure, error) {
	var exposures []*publishmodel.Exposure
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		exposures = append(exposures, m)
		return nil
	}); err != nil {
		return nil, err
	}

	return exposures, nil
}
