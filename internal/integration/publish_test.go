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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

func TestPublish(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env, client := testServer(t)
	db := env.Database()

	// Create an authorized app.
	aa := env.AuthorizedAppProvider()
	if err := aa.Add(ctx, &authorizedappmodel.AuthorizedApp{
		AppPackageName: "com.example.app",
		AllowedRegions: map[string]struct{}{
			"US": {},
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
	ec := &exportmodel.ExportConfig{
		BucketName:       "my-bucket",
		Period:           1 * time.Second,
		OutputRegion:     "US",
		From:             time.Now().Add(-2 * time.Second),
		Thru:             time.Now().Add(1 * time.Hour),
		SignatureInfoIDs: []int64{si.ID},
	}
	if err := exportdatabase.New(db).AddExportConfig(ctx, ec); err != nil {
		t.Fatal(err)
	}

	payload := &verifyapi.Publish{
		Keys:           util.GenerateExposureKeys(3, -1, false),
		Regions:        []string{"US"},
		AppPackageName: "com.example.app",

		// TODO: hook up verification
		VerificationPayload: "TODO",
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		t.Fatal(err)
	}

	resp, err := client.Post("/publish", "application/json", &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Ensure we get a successful response code.
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

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

	if got, want := len(exposures), 3; got != want {
		t.Errorf("expected %v to be %v: %#v", got, want, exposures)
	}

	// Create an export.
	resp, err = client.Get("/export/create-batches")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	resp, err = client.Get("/export/do-work")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// TODO: verify export has the correct file
	b, err := env.Blobstore().GetObject(ctx, "my-bucket", "index.txt")
	if err != nil {
		t.Fatal(err)
	}
	_ = b

	// TODO: verify signature
}
