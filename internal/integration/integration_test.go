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
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/export"
	exportpb "github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/project"
	publishdatabase "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/enkstest"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sethvargo/go-retry"
	"google.golang.org/protobuf/proto"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestEndToEnd(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	srv := enkstest.NewServer(t, testDatabaseInstance)

	// bootstrap the system
	bootstrap, err := enkstest.Bootstrap(ctx, srv.Env)
	if err != nil {
		t.Fatal(err)
	}

	// Grab common handles
	env := srv.Env
	db := env.Database()

	// Create the HTTP client
	client, err := newClient("http://" + srv.Server.Addr())
	if err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 1)
	if n, err := rand.Read(b); err != nil || n != 1 {
		t.Errorf("unable to randomly assign vaccine status")
	}
	vaccineStatus := (b[0]%2 == 1)

	// Publish some keys
	publishRequest := &verifyapi.Publish{
		Keys:              util.GenerateExposureKeys(12, -1, false),
		HealthAuthorityID: bootstrap.AuthorizedApp.AppPackageName,
		Vaccinated:        vaccineStatus,
	}

	verificationPayload, hmacKey := utils.IssueJWT(t, &utils.JWTConfig{
		HealthAuthority:      bootstrap.HealthAuthority,
		HealthAuthorityKey:   bootstrap.HealthAuthorityKey,
		ExposureKeys:         publishRequest.Keys,
		Key:                  bootstrap.Signer,
		ReportType:           verifyapi.ReportTypeConfirmed,
		SymptomOnsetInterval: uint32(publishmodel.IntervalNumber(time.Now().UTC().Add(-48 * time.Hour))),
	})

	publishRequest.VerificationPayload = verificationPayload
	publishRequest.HMACKey = hmacKey

	if _, err := client.Publish(ctx, publishRequest); err != nil {
		t.Fatal(err)
	}

	// Assert there are exposures in the database
	exposures, err := listExposures(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(exposures), 12; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}

	// Get the exported exposures
	var exportedKeys []*exportpb.TemporaryExposureKey
	backoff, _ := retry.NewConstant(500 * time.Millisecond)
	backoff = retry.WithMaxRetries(30, backoff)
	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		exportDir := bootstrap.ExportConfig.BucketName
		exportRoot := bootstrap.ExportConfig.FilenameRoot

		// Trigger an export
		if err := client.ExportCreateBatches(ctx); err != nil {
			return err
		}

		// Start export workers
		if err := client.ExportDoWork(ctx); err != nil {
			return err
		}

		// Attempt to get the index
		index, err := env.Blobstore().GetObject(ctx, exportDir, path.Join(exportRoot, "index.txt"))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return retry.RetryableError(fmt.Errorf("could not find index file: %w", err))
			}
			return err
		}

		// Find all files in the index
		var files []string
		lines := strings.Split(string(index), "\n")
		for _, entry := range lines {
			if strings.HasSuffix(entry, "zip") {
				files = append(files, entry)
			}
		}

		// Download export files contents
		var eks []*exportpb.TemporaryExposureKey
		for _, f := range files {
			data, err := env.Blobstore().GetObject(ctx, exportDir, f)
			if err != nil {
				return fmt.Errorf("failed to open %s/%s: %w", exportDir, f, err)
			}

			// Process contents as an export
			key, _, err := export.UnmarshalExportFile(data)
			if err != nil {
				return fmt.Errorf("failed to extract export data: %w", err)
			}

			// Try to marshal the result as JSON to verify its API compatible
			if _, err := json.Marshal(key); err != nil {
				return fmt.Errorf("failed to marshal as json: %w", err)
			}

			if got, want := *key.BatchSize, int32(1); got != want {
				return fmt.Errorf("expected %v to be %v", got, want)
			}
			if got, want := *key.BatchNum, int32(1); got != want {
				return fmt.Errorf("expected %v to be %v", got, want)
			}
			if got, want := *key.Region, "TEST"; got != want {
				return fmt.Errorf("expected %v to be %v", got, want)
			}
			eks = append(eks, key.Keys...)
		}
		exportedKeys = eks
		if got, want := len(exportedKeys), 12; got != want {
			return retry.RetryableError(fmt.Errorf("Not all keys are exported yet. Want: %d, got: %d", want, got))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Verify expected keys were exported
	wantKeys := exportedKeysFrom(t, publishRequest.Keys, publishRequest.Vaccinated)
	sortTEKs(wantKeys)
	sortTEKs(exportedKeys)
	opts := cmpopts.IgnoreUnexported(exportpb.TemporaryExposureKey{})
	if diff := cmp.Diff(exportedKeys, wantKeys, opts); diff != "" {
		t.Fatalf("wrong export keys (-got +want):\n%s", diff)
	}
}

// listExposures finds all exposures in the database.
func listExposures(ctx context.Context, db *database.DB) ([]*publishmodel.Exposure, error) {
	criteria := publishdatabase.IterateExposuresCriteria{
		OnlyLocalProvenance: false,
	}

	var exposures []*publishmodel.Exposure
	if _, err := publishdatabase.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
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
func exportedKeysFrom(tb testing.TB, keys []verifyapi.ExposureKey, vaccineStatus bool) []*exportpb.TemporaryExposureKey {
	s := make([]*exportpb.TemporaryExposureKey, len(keys))
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].IntervalNumber < keys[j].IntervalNumber
	})
	daysSince := int32(-len(keys) + 2)
	for i, key := range keys {
		decoded, err := base64util.DecodeString(key.Key)
		if err != nil {
			tb.Fatalf("failed to decode %v: %v", key.Key, err)
		}

		s[i] = &exportpb.TemporaryExposureKey{
			KeyData:                    decoded,
			TransmissionRiskLevel:      proto.Int32(int32(key.TransmissionRisk)),
			RollingStartIntervalNumber: proto.Int32(key.IntervalNumber),
			ReportType:                 exportpb.TemporaryExposureKey_CONFIRMED_TEST.Enum(),
			// Keys are generated 1 day ago and then -1 day for each additional.
			DaysSinceOnsetOfSymptoms: proto.Int32(daysSince),
			Vaccinated:               proto.Bool(vaccineStatus),
		}
		daysSince++
	}

	sortTEKs(s)
	return s
}

// sortTEKs sorts a collection of TEKs by their key data, useful in tests for
// comparing.
func sortTEKs(keys []*exportpb.TemporaryExposureKey) {
	sort.Slice(keys, func(i, j int) bool {
		return string(keys[i].KeyData) < string(keys[j].KeyData)
	})
}
