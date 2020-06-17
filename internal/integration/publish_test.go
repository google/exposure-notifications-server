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
	"fmt"
	"strings"
	"testing"
	"time"

	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/database"
	exportapi "github.com/google/exposure-notifications-server/internal/export"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/pb/export"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestCleanup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	env, client := testServer(t)
	db := env.Database()
	enClient := &EnServerClient{client: client}

	exportPeriod := 2 * time.Second
	criteria := publishdb.IterateExposuresCriteria{
		OnlyLocalProvenance: false,
	}

	// Create an authorized app.
	startAuthorizedApp(ctx, env, t)

	// Create a signature info.
	createSignatureInfo(ctx, db, exportPeriod, t)

	firstPayload := newPayloads(3)
	enClient.PublishKeys(t, firstPayload)
	assertExposures(t, ctx, db, 3)
	firstBatchKeys := sets.NewString()
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		firstBatchKeys.Insert(string(m.ExposureKey))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	secondPayload := newPayloads(3)
	enClient.PublishKeys(t, secondPayload)
	assertExposures(t, ctx, db, 6)

	// Move the tick of first batch, and make them old
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		keyStr := string(m.ExposureKey)
		if !firstBatchKeys.Has(keyStr) {
			return nil
		}
		if _, err := publishdb.New(db).DeleteExposure(ctx, m.ExposureKey); err != nil {
			return fmt.Errorf("failed deleting exposure %v: %v", m.ExposureKey, err)
		}
		m.CreatedAt = m.CreatedAt.Add(-366 * time.Hour)
		if err := publishdb.New(db).InsertExposures(ctx, []*publishmodel.Exposure{m}); err != nil {
			return fmt.Errorf("failed inserting exposure %v with updated creation time: %v",
				m.ExposureKey, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	enClient.CleanupExposures(t)
	assertExposures(t, ctx, db, 3)
}

func TestPublish(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	env, client := testServer(t)
	db := env.Database()
	enClient := &EnServerClient{client: client}

	exportPeriod := 2 * time.Second

	// Create an authorized app.
	startAuthorizedApp(ctx, env, t)

	// Create a signature info.
	createSignatureInfo(ctx, db, exportPeriod, t)

	payload := newPayloads(3)

	enClient.PublishKeys(t, payload)
	assertExposures(t, ctx, db, 3)

	t.Logf("Waiting %v before export batches", exportPeriod+1*time.Second)
	time.Sleep(exportPeriod + 1*time.Second)
	enClient.ExportBatches(t)

	t.Logf("Waiting %v before starting workers", 500*time.Millisecond)
	time.Sleep(500 * time.Millisecond)
	enClient.StartExportWorkers(t)

	keyExport := getKeysFromLatestBatch(t, "my-bucket", ctx, env)

	got := keyExport

	wantedKeysMap := make(map[string]*export.TemporaryExposureKey)
	for _, key := range payload.Keys {
		wantedKeysMap[key.Key] = &export.TemporaryExposureKey{
			KeyData:                    util.DecodeKey(key.Key),
			TransmissionRiskLevel:      proto.Int32(int32(key.TransmissionRisk)),
			RollingStartIntervalNumber: proto.Int32(key.IntervalNumber),
		}
	}

	want := &export.TemporaryExposureKeyExport{
		Region:    proto.String("TEST"),
		BatchNum:  proto.Int32(1),
		BatchSize: proto.Int32(1),
	}

	if *want.BatchSize != *got.BatchSize {
		t.Errorf("Invalid BatchSize: want: %v, got: %v", *want.BatchSize, *got.BatchSize)
	}

	if *want.BatchNum != *got.BatchNum {
		t.Errorf("Invalid BatchNum: want: %v, got: %v", *want.BatchNum, *got.BatchNum)
	}

	if *want.Region != *got.Region {
		t.Errorf("Invalid Region: want: %v, got: %v", *want.BatchSize, *got.BatchSize)
	}

	for _, key := range got.Keys {
		s := util.ToBase64(key.KeyData)
		wantedKey := wantedKeysMap[s]
		diff := cmp.Diff(wantedKey, key, cmpopts.IgnoreUnexported(export.TemporaryExposureKey{}))
		if diff != "" {
			t.Errorf("invalid key value: %v:%v", s, diff)
		}
	}

	bytes, err := json.MarshalIndent(got, "", "")
	if err != nil {
		t.Fatalf("can't marshal json results: %v", err)
	}

	t.Logf("%v", string(bytes))
	// TODO: verify signature
}

func startAuthorizedApp(ctx context.Context, env *serverenv.ServerEnv, t *testing.T) {
	aa := env.AuthorizedAppProvider()
	if err := aa.Add(ctx, &authorizedappmodel.AuthorizedApp{
		AppPackageName: "com.example.app",
		AllowedRegions: map[string]struct{}{
			"TEST": {},
		},
		AllowedHealthAuthorityIDs: map[int64]struct{}{
			12345: {},
		},

		// TODO: hook up verification, and disable
		BypassHealthAuthorityVerification: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func createSignatureInfo(ctx context.Context, db *database.DB, exportPeriod time.Duration, t *testing.T) {
	si := &exportmodel.SignatureInfo{
		SigningKey:        "signer",
		SigningKeyVersion: "v1",
		SigningKeyID:      "US",
	}
	if err := exportdatabase.New(db).AddSignatureInfo(ctx, si); err != nil {
		t.Fatal(err)
	}

	// Create an export config.
	exportPeriod := 2 * time.Second
	ec := &exportmodel.ExportConfig{
		BucketName:       "my-bucket",
		Period:           exportPeriod,
		OutputRegion:     "TEST",
		From:             time.Now().Add(-2 * time.Second),
		Thru:             time.Now().Add(1 * time.Hour),
		SignatureInfoIDs: []int64{},
	}
	if err := exportdatabase.New(db).AddExportConfig(ctx, ec); err != nil {
		t.Fatal(err)
	}
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

func getKeysFromAllBatches(t *testing.T, exportDir string, ctx context.Context, env *serverenv.ServerEnv) []*export.TemporaryExposureKeyExport {
	var res []*export.TemporaryExposureKeyExport
	exportFiles := getAllBatchesFiles(t, exportDir, ctx, env)
	t.Log("all files:", exportFiles)

	for _, exportFile := range exportFiles {
		e := readKeyExportFromBlob(t, exportDir, exportFile, ctx, env)
		res = append(res, e)
	}
	return res
}

func getKeysFromLatestBatch(t *testing.T, exportDir string, ctx context.Context, env *serverenv.ServerEnv) *export.TemporaryExposureKeyExport {
	files := getAllBatchesFiles(t, exportDir, ctx, env)
	exportFile := getLatestFile(files)
	t.Log("latest:", exportFile)
	return readKeyExportFromBlob(t, exportDir, exportFile, ctx, env)
}

func readKeyExportFromBlob(t *testing.T, exportDir, exportFile string, ctx context.Context, env *serverenv.ServerEnv) *export.TemporaryExposureKeyExport {
	if exportFile == "" {
		t.Fatalf("Can't find export files in blobstore: %q", exportDir)
	}

	t.Logf("Reading keys data from: %q", exportFile)

	blob, err := env.Blobstore().GetObject(ctx, exportDir, exportFile)
	if err != nil {
		t.Fatalf("can't marshal json results: %v", err)
	}

	keyExport, err := exportapi.UnmarshalExportFile(blob)
	if err != nil {
		t.Fatalf("can't extract export data: %v", err)
	}

	return keyExport
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
