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
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	exportapi "github.com/google/exposure-notifications-server/internal/export"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/pb/export"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/internal/util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"

	pgx "github.com/jackc/pgx/v4"
)

func TestCleanupExposure(t *testing.T) {
	t.Parallel()
	var (
		ctx          = context.Background()
		exportPeriod = 2 * time.Second
		criteria     = publishdb.IterateExposuresCriteria{
			OnlyLocalProvenance: false,
		}
	)

	_, enClient, db := TestServer(t, ctx, exportPeriod)

	firstPayload := newPayloads(3)
	firstBatchKeys := keysTransformation(t, firstPayload.Keys)

	enClient.PublishKeys(t, firstPayload)
	// This is one time check to ensure the logic of `keysTransformation` is
	// consistent with business logic in the server
	assertKeysTransformation(t, ctx, db, criteria, firstBatchKeys)

	assertExposures(t, ctx, db, 3)

	secondPayload := newPayloads(3)
	enClient.PublishKeys(t, secondPayload)
	assertExposures(t, ctx, db, 6)

	alterExposureCreatedAt(t, ctx, db, criteria, firstBatchKeys, -366*time.Hour)
	enClient.CleanupExposures(t)
	assertExposures(t, ctx, db, 3)
}

func TestCleanupExport(t *testing.T) {
	t.Parallel()

	var (
		ctx          = context.Background()
		exportPeriod = 2 * time.Second
	)

	env, enClient, db := TestServer(t, ctx, exportPeriod)

	firstPayload := newPayloads(3)
	enClient.PublishKeys(t, firstPayload)
	assertExposures(t, ctx, db, 3)
	exportKeys(t, enClient, exportPeriod)
	timeAfterFirstExport := time.Now()
	keyExport := getKeysFromLatestBatch(t, exportDir, ctx, env)
	got := keyExport
	wantedKeysMap, wantedExport := wantFromKeys(firstPayload.Keys, 1, 1)
	assertExports(t, wantedKeysMap, wantedExport, got)

	var firstBatchExportFile *exportmodel.ExportFile
	firstBatchFiles := getAllBatchesFiles(t, exportDir, ctx, env)
	firstBatchExportFile, err := exportdatabase.New(db).LookupExportFile(ctx, firstBatchFiles[0])
	if err != nil {
		t.Fatalf("Failed to look up exportfile %q: %v", firstBatchFiles[0], err)
	}

	secondPayload := newPayloads(3)
	enClient.PublishKeys(t, secondPayload)
	assertExposures(t, ctx, db, 6)
	exportKeys(t, enClient, exportPeriod)
	keyExport = getKeysFromLatestBatch(t, exportDir, ctx, env)
	wantedKeysMap, wantedExport = wantFromKeys(secondPayload.Keys, 1, 1)
	assertExports(t, wantedKeysMap, wantedExport, keyExport)

	// Move the tick of first batch export to be old
	if err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
		UPDATE
			ExportBatch
		SET
			start_timestamp = $1, end_timestamp = $2
		WHERE
			batch_id = $3
		`, timeAfterFirstExport.Add(-367*time.Hour),
			timeAfterFirstExport.Add(-366*time.Hour),
			firstBatchExportFile.BatchID,
		)
		return err
	}); err != nil {
		t.Fatalf("Failed updating batch_id %d: %v", firstBatchExportFile.BatchID, err)
	}

	// Make sure keys were exported before cleanup
	exposureKeys := getKeysFromAllBatches(t, exportDir, ctx, env)
	wantedKeysMap, _ = wantFromKeys(append(firstPayload.Keys, secondPayload.Keys...), 1, 1)
	assertExportedKeysMatch(t, wantedKeysMap, exposureKeys)

	enClient.CleanupExports(t)
	assertExposures(t, ctx, db, 6)
	var exposureKeysAfterCleanup []*export.TemporaryExposureKeyExport
	exportFiles := getAllBatchesFiles(t, exportDir, ctx, env)
	for _, exportFile := range exportFiles {
		e, err := readKeyExportFromBlob(t, exportDir, exportFile, ctx, env)
		if exportFile == firstBatchExportFile.Filename {
			if err == nil {
				t.Fatalf("File %q should have been deleted", exportFile)
			}
			if errors.Is(err, storage.ErrNotFound) {
				continue // This is expected, as the filename of first batch is not cleaned up from "index.txt"
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		exposureKeysAfterCleanup = append(exposureKeysAfterCleanup, e)
	}
	wantedKeysMap, _ = wantFromKeys(secondPayload.Keys, 1, 1)
	assertExportedKeysMatch(t, wantedKeysMap, exposureKeysAfterCleanup)
	assertExposures(t, ctx, db, 6)
}

func TestPublish(t *testing.T) {
	t.Parallel()

	var (
		ctx          = context.Background()
		exportPeriod = 2 * time.Second
	)

	env, enClient, db := TestServer(t, ctx, exportPeriod)

	payload := newPayloads(3)

	enClient.PublishKeys(t, payload)
	assertExposures(t, ctx, db, 3)

	exportKeys(t, enClient, exportPeriod)
	keyExport := getKeysFromLatestBatch(t, exportDir, ctx, env)
	got := keyExport
	wantedKeysMap, wantedExport := wantFromKeys(payload.Keys, 1, 1)
	assertExports(t, wantedKeysMap, wantedExport, got)
	// TODO: verify signature
}

func exportKeys(t *testing.T, enClient *EnServerClient, exportPeriod time.Duration) {
	t.Logf("Waiting %v before export batches", exportPeriod+1*time.Second)
	time.Sleep(exportPeriod + 1*time.Second)
	enClient.ExportBatches(t)

	t.Logf("Waiting %v before starting workers", 500*time.Millisecond)
	time.Sleep(500 * time.Millisecond)
	enClient.StartExportWorkers(t)
}

// keysTransformation transforms []verifyapi.ExposureKey to string keys, this
// function assumes that server saves keys with the same method. This can be
// verified by `assertKeysTransformation` function below
func keysTransformation(t *testing.T, keys []verifyapi.ExposureKey) map[string]bool {
	res := make(map[string]bool)
	for _, p := range keys {
		dec, err := base64util.DecodeString(p.Key)
		if err != nil {
			t.Fatalf("Failed to decode key %q: %v", p.Key, err)
		}
		res[string(dec)] = true
	}
	return res
}

func assertKeysTransformation(t *testing.T, ctx context.Context, db *database.DB,
	criteria publishdb.IterateExposuresCriteria, generatedKeys map[string]bool) {
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		if _, ok := generatedKeys[string(m.ExposureKey)]; ok {
			generatedKeys[string(m.ExposureKey)] = false
		} else {
			return fmt.Errorf("key %q doesn't belong to generated keys: %v", string(m.ExposureKey), generatedKeys)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func alterExposureCreatedAt(t *testing.T, ctx context.Context, db *database.DB,
	criteria publishdb.IterateExposuresCriteria, keys map[string]bool, timeDelta time.Duration) {
	// Move the tick of first batch, and make them old
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		if _, ok := keys[string(m.ExposureKey)]; !ok {
			return nil
		}
		if _, err := publishdb.New(db).DeleteExposure(ctx, m.ExposureKey); err != nil {
			return fmt.Errorf("failed deleting exposure %v: %v", m.ExposureKey, err)
		}
		m.CreatedAt = m.CreatedAt.Add(timeDelta)
		if err := publishdb.New(db).InsertExposures(ctx, []*publishmodel.Exposure{m}); err != nil {
			return fmt.Errorf("failed inserting exposure %v with updated creation time: %v",
				m.ExposureKey, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func assertExports(t *testing.T, wantedKeysMap map[string]*export.TemporaryExposureKey,
	wantExport *export.TemporaryExposureKeyExport, got *export.TemporaryExposureKeyExport) {
	if wantExport != nil {
		if *wantExport.BatchSize != *got.BatchSize {
			t.Errorf("Invalid BatchSize: want: %v, got: %v", *wantExport.BatchSize, *got.BatchSize)
		}

		if *wantExport.BatchNum != *got.BatchNum {
			t.Errorf("Invalid BatchNum: want: %v, got: %v", *wantExport.BatchNum, *got.BatchNum)
		}

		if *wantExport.Region != *got.Region {
			t.Errorf("Invalid Region: want: %v, got: %v", *wantExport.BatchSize, *got.BatchSize)
		}
	}

	for _, key := range got.Keys {
		s := util.ToBase64(key.KeyData)
		wantedKey := wantedKeysMap[s]
		diff := cmp.Diff(wantedKey, key, cmpopts.IgnoreUnexported(export.TemporaryExposureKey{}))
		if diff != "" {
			t.Errorf("invalid key value: %v:%v", s, diff)
		}
	}

	if _, err := json.MarshalIndent(got, "", ""); err != nil {
		t.Fatalf("can't marshal json results: %v", err)
	}
}

func assertExportedKeysMatch(t *testing.T, wantedKeysMap map[string]*export.TemporaryExposureKey,
	got []*export.TemporaryExposureKeyExport) {
	var gotKeys []*export.TemporaryExposureKey
	for _, e := range got {
		gotKeys = append(gotKeys, e.Keys...)
	}
	if len(wantedKeysMap) != len(gotKeys) {
		t.Fatalf("Number of keys mismatch. Want: %d, got %d", len(wantedKeysMap), len(gotKeys))
	}
	for _, key := range gotKeys {
		s := util.ToBase64(key.KeyData)
		wantedKey := wantedKeysMap[s]
		diff := cmp.Diff(wantedKey, key, cmpopts.IgnoreUnexported(export.TemporaryExposureKey{}))
		if diff != "" {
			t.Errorf("invalid key value: %v:%v", s, diff)
		}
	}

	if _, err := json.MarshalIndent(got, "", ""); err != nil {
		t.Fatalf("can't marshal json results: %v", err)
	}
}

func wantFromKeys(keys []verifyapi.ExposureKey, batchNum, batchSize int32) (map[string]*export.TemporaryExposureKey,
	*export.TemporaryExposureKeyExport) {
	wantedKeysMap := make(map[string]*export.TemporaryExposureKey)
	for _, key := range keys {
		wantedKeysMap[key.Key] = &export.TemporaryExposureKey{
			KeyData:                    util.DecodeKey(key.Key),
			TransmissionRiskLevel:      proto.Int32(int32(key.TransmissionRisk)),
			RollingStartIntervalNumber: proto.Int32(key.IntervalNumber),
		}
	}

	wantExport := &export.TemporaryExposureKeyExport{
		Region:    proto.String("TEST"),
		BatchNum:  proto.Int32(batchNum),
		BatchSize: proto.Int32(batchSize),
	}

	return wantedKeysMap, wantExport
}

func newPayloads(n int) verifyapi.Publish {
	return verifyapi.Publish{
		Keys:           util.GenerateExposureKeys(3, -1, false),
		Regions:        []string{"TEST"},
		AppPackageName: "com.example.app",

		// TODO: hook up verification
		VerificationPayload: "TODO",
	}
}

func assertExposures(t *testing.T, ctx context.Context, db *database.DB, want int) {
	// Look up the exposures in the database.
	criteria := publishdb.IterateExposuresCriteria{
		OnlyLocalProvenance: false,
	}

	var exposures []*publishmodel.Exposure
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		exposures = append(exposures, m)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	got := len(exposures)
	if want != got {
		t.Errorf("Want: %d, got: %d", want, got)
	}
}

// getKeysFromAllBatches gets all exported keys, unless the filename is
// explicitly listed in excluded
func getKeysFromAllBatches(t *testing.T, exportDir string, ctx context.Context, env *serverenv.ServerEnv) []*export.TemporaryExposureKeyExport {
	var res []*export.TemporaryExposureKeyExport
	exportFiles := getAllBatchesFiles(t, exportDir, ctx, env)
	t.Log("all files:", exportFiles)

	for _, exportFile := range exportFiles {
		e, err := readKeyExportFromBlob(t, exportDir, exportFile, ctx, env)
		if err != nil {
			t.Fatalf("cannot read file %q: %v", exportFile, err)
		}
		res = append(res, e)
	}
	return res
}

func getKeysFromLatestBatch(t *testing.T, exportDir string, ctx context.Context, env *serverenv.ServerEnv) *export.TemporaryExposureKeyExport {
	files := getAllBatchesFiles(t, exportDir, ctx, env)
	exportFile := getLatestFile(files)
	t.Log("latest:", exportFile)
	res, err := readKeyExportFromBlob(t, exportDir, exportFile, ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func readKeyExportFromBlob(t *testing.T, exportDir, exportFile string, ctx context.Context, env *serverenv.ServerEnv) (*export.TemporaryExposureKeyExport, error) {
	if exportFile == "" {
		return nil, fmt.Errorf("Can't find export files in blobstore: %q", exportDir)
	}

	t.Logf("Reading keys data from: %q", exportFile)

	blob, err := env.Blobstore().GetObject(ctx, exportDir, exportFile)
	if err != nil {
		return nil, fmt.Errorf("can't open file from %q in dir %q: %w", exportFile, exportDir, err)
	}

	keyExport, err := exportapi.UnmarshalExportFile(blob)
	if err != nil {
		return nil, fmt.Errorf("can't extract export data: %w", err)
	}

	return keyExport, nil
}

func getLatestFile(files []string) string {
	latestFileName := ""
	for _, fileName := range files {
		if strings.HasSuffix(fileName, "zip") {
			if latestFileName == "" {
				latestFileName = fileName
			} else {
				if fileName > latestFileName {
					latestFileName = fileName
				}
			}
		}
	}

	return latestFileName
}

func getAllBatchesFiles(t *testing.T, exportDir string, ctx context.Context, env *serverenv.ServerEnv) []string {
	readmeBlob, err := env.Blobstore().GetObject(ctx, exportDir, "index.txt")
	if err != nil {
		t.Fatalf("Can't file index.txt in blobstore: %v", err)
	}
	t.Log("index.txt: ", string(readmeBlob))
	return strings.Split(string(readmeBlob), "\n")
}
