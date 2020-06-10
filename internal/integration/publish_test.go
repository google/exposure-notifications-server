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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestPublish(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Export
	exportConfig := &exportapi.Config{
		CreateTimeout:  10 * time.Second,
		WorkerTimeout:  10 * time.Second,
		MinRecords:     1,
		PaddingRange:   1,
		MaxRecords:     10000,
		TruncateWindow: 1 * time.Millisecond,
		MinWindowAge:   1 * time.Second,
		TTL:            336 * time.Hour,
	}

	env, client := testServer(t, exportConfig)
	db := env.Database()
	server := &EnServerClient{client: client}

	// Create an authorized app.
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

	// Create a signature info.
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

	payload := verifyapi.Publish{
		Keys:           util.GenerateExposureKeys(3, -1, false),
		Regions:        []string{"TEST"},
		AppPackageName: "com.example.app",

		// TODO: hook up verification
		VerificationPayload: "TODO",
	}

	server.PublishKeys(t, payload)

	// Look up the exposures in the database.
	criteria := publishdb.IterateExposuresCriteria{
		OnlyLocalProvenance: false,
	}

	var exposures []*publishmodel.Exposure
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		t.Logf("NEW EXPOSURE: %v", m)
		exposures = append(exposures, m)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if got, want := len(exposures), 3; got != want {
		t.Errorf("expected %v to be %v: %#v", got, want, exposures)
	}

	wait(t, exportPeriod+500*time.Millisecond, "Waiting before export batches")
	server.ExportBatches(t)

	wait(t, 500*time.Millisecond, "Waiting before staring workers")
	server.StartExportWorkers(t)

	memory, ok := env.Blobstore().(*storage.Memory)
	if !ok {
		t.Fatalf("can't use %t blobstore for verification", env.Blobstore())
	}
	keyExport := getKeysFromLatestBatch(t, "my-bucket", ctx, env, memory)

	got := keyExport

	wantedKeysMap := make(map[string]export.TemporaryExposureKey)
	for _, key := range payload.Keys {
		wantedKeysMap[key.Key] = export.TemporaryExposureKey{
			KeyData:                    util.DecodeKey(key.Key),
			TransmissionRiskLevel:      proto.Int32(int32(key.TransmissionRisk)),
			RollingStartIntervalNumber: proto.Int32(key.IntervalNumber),
		}
	}

	want := export.TemporaryExposureKeyExport{
		StartTimestamp: nil,
		EndTimestamp:   nil,
		Region:         proto.String("TEST"),
		BatchNum:       proto.Int32(1),
		BatchSize:      proto.Int32(1),
		SignatureInfos: nil,
		Keys:           nil,
	}

	options := []cmp.Option{
		cmpopts.IgnoreFields(want, "StartTimestamp"),
		cmpopts.IgnoreFields(want, "EndTimestamp"),
		cmpopts.IgnoreFields(want, "SignatureInfos"),
		cmpopts.IgnoreFields(want, "Keys"),
		cmpopts.IgnoreUnexported(want),
	}

	diff := cmp.Diff(got, &want, options...)
	if diff != "" {
		t.Errorf("%v", diff)
	}

	for _, key := range got.Keys {
		s := util.ToBase64(key.KeyData)
		wantedKey := wantedKeysMap[s]
		gotKey := *key
		diff := cmp.Diff(wantedKey, gotKey, cmpopts.IgnoreUnexported(gotKey))
		if diff != "" {

			t.Logf("WANT: %v", proto.MarshalTextString(&wantedKey))
			t.Logf(" GOT: %v", proto.MarshalTextString(&gotKey))

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

func getKeysFromLatestBatch(t *testing.T, exportDir string, ctx context.Context, env *serverenv.ServerEnv, memory *storage.Memory) *export.TemporaryExposureKeyExport {
	exportFile := getLatestFile(t, memory, ctx, exportDir)

	t.Logf("Reading keys data from: %v", exportFile)

	blob, err := env.Blobstore().GetObject(ctx, "", exportFile)
	if err != nil {
		t.Fatal(err)
	}

	keyExport, err := exportapi.UnmarshalExportFile(blob)
	if err != nil {
		t.Fatalf("can't extract export data: %v", err)
	}

	return keyExport
}

func getLatestFile(t *testing.T, blobstore *storage.Memory, ctx context.Context, exportDir string) string {
	files := blobstore.ListObjects(ctx, exportDir)

	archiveFiles := make([]string, 0)
	for fileName := range files {
		if strings.HasSuffix(fileName, "zip") {
			archiveFiles = append(archiveFiles, fileName)
		}
	}

	if len(archiveFiles) < 1 {
		t.Fatalf("can't find export archives in %v", exportDir)
	}

	sort.SliceStable(archiveFiles, func(i, j int) bool {
		return archiveFiles[i] > archiveFiles[j]
	})
	exportFile := archiveFiles[0]
	return exportFile
}

func wait(t *testing.T, duration time.Duration, message string) {
	t.Logf("%s - waiting for %v", message, duration)
	time.Sleep(duration)
}
