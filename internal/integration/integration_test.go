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

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	exportapi "github.com/google/exposure-notifications-server/internal/export"
	exportdb "github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/pb/export"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/storage"
	testutil "github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	pgx "github.com/jackc/pgx/v4"
	"github.com/sethvargo/go-retry"
	"google.golang.org/protobuf/proto"
)

func TestIntegration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name    string
		JWTWrap time.Duration
		Error   string
		WantErr bool
	}{
		{
			Name:    "exposure_verified",
			JWTWrap: time.Duration(0),
		},
		{
			Name:    "exposure_not_verified",
			JWTWrap: time.Hour,
			Error:   `"error":"unable to validate diagnosis verification: Token used before issued","code":"health_authority_verification_certificate_invalid"`,
			WantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc // Capture test case var for parallel runs

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			env, client := NewTestServer(t)
			db := env.Database()
			jwtCfg, exportDir, exportRoot, appName := Seed(t, ctx, db, 2*time.Second)

			// Set query criteria (used throughout)
			criteria := publishdb.IterateExposuresCriteria{
				OnlyLocalProvenance: false,
			}

			keys := util.GenerateExposureKeys(3, -1, false)

			// Publish 3 keys
			payload := &verifyapi.Publish{
				Keys:              keys,
				HealthAuthorityID: appName,
			}
			jwtCfg.ExposureKeys = keys
			jwtCfg.JWTWarp = tc.JWTWrap
			verification, salt := testutil.IssueJWT(t, jwtCfg)
			payload.VerificationPayload = verification
			payload.HMACKey = salt
			var revisionToken string
			resp, err := client.PublishKeys(payload)

			if tc.WantErr {
				if err == nil || !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("expected error: %v, got: %v", tc.Error, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if resp.RevisionToken == "" {
				t.Fatal("empty revision token")
			}
			revisionToken = resp.RevisionToken
			t.Logf("response: %+v", resp)

			// Assert there are 3 exposures in the database
			{
				exposures, err := getExposures(db, criteria)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := len(exposures), 3; got != want {
					t.Fatalf("expected %#v to be %#v", got, want)
				}
			}

			// Get the exported exposures
			var exported *export.TemporaryExposureKeyExport
			Eventually(t, 30, func() error {
				// Trigger an export
				if err := client.ExportBatches(); err != nil {
					return err
				}

				// Start export workers
				if err := client.StartExportWorkers(); err != nil {
					return err
				}

				// Attempt to get the index
				index, err := env.Blobstore().GetObject(ctx, exportDir, IndexFilePath(exportRoot))
				if err != nil {
					if errors.Is(err, storage.ErrNotFound) {
						return retry.RetryableError(fmt.Errorf("Can not find index file: %v", err))
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
					return retry.RetryableError(fmt.Errorf("failed to find latest export"))
				}

				// Download the latest export file contents
				data, err := env.Blobstore().GetObject(ctx, exportDir, latest)
				if err != nil {
					return fmt.Errorf("failed to open %s/%s: %w", exportDir, latest, err)
				}

				// Process contents as an export
				key, _, err := exportapi.UnmarshalExportFile(data)
				if err != nil {
					return fmt.Errorf("failed to extract export data: %w", err)
				}

				exported = key
				return nil
			})
			// Sort keys for predictable testing
			sortTEKs(exported.Keys)

			// Verify export is correct
			if got, want := *exported.BatchSize, int32(1); got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
			if got, want := *exported.BatchNum, int32(1); got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
			if got, want := *exported.Region, "TEST"; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}

			// Verify expected keys were exported
			opts := cmpopts.IgnoreUnexported(export.TemporaryExposureKey{})
			if diff := cmp.Diff(exported.Keys, exportedKeysFrom(t, payload.Keys), opts); diff != "" {
				t.Errorf("wrong export keys (-got +want):\n%s", diff)
			}

			// Try to marshal the result as JSON to verify its API compatible
			if _, err := json.Marshal(exported); err != nil {
				t.Errorf("failed to marshal as json: %v", err)
			}

			// TODO: verify signature

			// Mark the first key as old so it'll be cleaned up
			if err := db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
				result, err := tx.Exec(ctx, `
				WITH cte AS (
					SELECT exposure_key
					FROM Exposure
					LIMIT 1
				)

				UPDATE
					Exposure e
				SET
					created_at = $1
				FROM cte
				WHERE e.exposure_key = cte.exposure_key
			`,
					time.Now().Add(-30*24*time.Hour),
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

			// Perform cleanup
			if err := client.CleanupExposures(); err != nil {
				t.Fatal(err)
			}

			// Assert there are 2 exposures in the database now
			{
				exposures, err := getExposures(db, criteria)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := len(exposures), 2; got != want {
					t.Fatalf("expected %v to be %v", got, want)
				}
			}

			// Rotate Keys. Should Genereate a new key.
			time.Sleep(200 * time.Millisecond) // Ensure DeleteOldKeyPeriod is elapsed
			if err := client.RotateKeys(); err != nil {
				t.Fatalf("Error rotating keys: %v", err)
			}

			// Rotate Keys. Should Delete the original key.
			time.Sleep(200 * time.Millisecond) // Ensure DeleteOldKeyPeriod is elapsed
			if err := client.RotateKeys(); err != nil {
				t.Fatalf("Error rotating keys: %v", err)
			}

			// Re-publish with the original token. This key is now not-allowed.
			payload.RevisionToken = revisionToken
			if _, err := client.PublishKeys(payload); err == nil {
				t.Fatal(err)
			} else if !strings.Contains(err.Error(), verifyapi.ErrorInvalidRevisionToken) {
				t.Fatal(err)
			}

			// Publish some new keys so we can generate a new batch
			keys = util.GenerateExposureKeys(3, -1, false)
			payload.Keys = keys
			jwtCfg.ExposureKeys = keys
			jwtCfg.JWTWarp = tc.JWTWrap
			verification, salt = testutil.IssueJWT(t, jwtCfg)
			payload.VerificationPayload = verification
			payload.HMACKey = salt
			if resp, err := client.PublishKeys(payload); err != nil {
				t.Fatal(err)
			} else {
				t.Logf("response: %+v", resp)
			}

			// Assert there are 5 exposures in the database
			{
				exposures, err := getExposures(db, criteria)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := len(exposures), 5; got != want {
					t.Fatalf("expected %v to be %v", got, want)
				}
			}

			// Wait for the export to be created and get the list of files
			var batchFiles []string
			Eventually(t, 30, func() error {
				// Trigger an export
				if err := client.ExportBatches(); err != nil {
					return err
				}

				// Start export workers
				if err := client.StartExportWorkers(); err != nil {
					return err
				}

				// Attempt to get the index
				index, err := env.Blobstore().GetObject(ctx, exportDir, IndexFilePath(exportRoot))
				if err != nil {
					if errors.Is(err, storage.ErrNotFound) {
						return retry.RetryableError(err)
					}
					return err
				}

				// Ensure the new export is created
				batchFiles = strings.Split(string(index), "\n")
				switch len(batchFiles) {
				case 0:
					return fmt.Errorf("somehow there are no exports?")
				case 1:
					return retry.RetryableError(fmt.Errorf("next export does not exist yet"))
				case 2:
				default:
					return fmt.Errorf("expected 2 exports, got %d", len(batchFiles))
				}
				return nil
			})

			// Find the export file that contains the batch - we need this to modify the
			// batch later to force it to cleanup.
			exportFile, err := exportdb.New(db).LookupExportFile(ctx, batchFiles[0])
			if err != nil {
				t.Fatal(err)
			}

			// Mark the export in the past to force a cleanup
			if err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
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
			Eventually(t, 30, func() error {
				// Trigger cleanup
				if err := client.CleanupExports(); err != nil {
					return err
				}

				// Attempt to get the index
				index, err := env.Blobstore().GetObject(ctx, exportDir, IndexFilePath(exportRoot))
				if err != nil {
					if errors.Is(err, storage.ErrNotFound) {
						return retry.RetryableError(err)
					}
					return err
				}

				// Ensure the new export is created
				batchFiles = strings.Split(string(index), "\n")
				for _, f := range batchFiles {
					if f != exportFile.Filename {
						continue
					}

					// Lookup the file, hope it's gone
					if _, err := env.Blobstore().GetObject(ctx, exportDir, f); err != nil {
						if errors.Is(err, storage.ErrNotFound) {
							return nil // expected
						} else {
							return err
						}
					}

					return retry.RetryableError(fmt.Errorf("export file still exists"))
				}

				return nil
			})
		})
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

// exportedKeysFrom constructs valid TEKs from the given exposure keys. This is
// mostly used for testing and comparing that two expected sets match (input and
// output).
func exportedKeysFrom(tb testing.TB, keys []verifyapi.ExposureKey) []*export.TemporaryExposureKey {
	s := make([]*export.TemporaryExposureKey, len(keys))
	for i, key := range keys {
		decoded, err := base64util.DecodeString(key.Key)
		if err != nil {
			tb.Fatalf("failed to decode %v: %v", key.Key, err)
		}

		s[i] = &export.TemporaryExposureKey{
			KeyData:                    decoded,
			TransmissionRiskLevel:      proto.Int32(int32(key.TransmissionRisk)),
			RollingStartIntervalNumber: proto.Int32(key.IntervalNumber),
			ReportType:                 export.TemporaryExposureKey_CONFIRMED_TEST.Enum(),
		}
	}

	sortTEKs(s)
	return s
}

// sortTEKs sorts a collection of TEKs by their key data, useful in tests for
// comparing.
func sortTEKs(keys []*export.TemporaryExposureKey) {
	sort.Slice(keys, func(i, j int) bool {
		return string(keys[i].KeyData) < string(keys[j].KeyData)
	})
}
