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

package exportimport

import (
	"sync"
	"testing"
	"time"

	exportproto "github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/project"
	pubmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
)

type keyGenerator struct {
	pos   int
	count int8
	mu    sync.Mutex
}

func (k *keyGenerator) fakeExposureKey(t testing.TB) []byte {
	t.Helper()
	k.mu.Lock()
	defer k.mu.Unlock()

	k.count++
	if k.count < 0 {
		k.count = 0
		k.pos++
		if k.pos >= 15 {
			t.Fatal("overflow, too many keys generated")
		}
	}

	key := make([]byte, 16)
	key[k.pos] = byte(k.count)
	return key
}

func TestTransform(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		traveler bool
	}{
		{
			name:     "not_travler",
			traveler: false,
		},
		{
			name:     "traveler",
			traveler: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := project.TestContext(t)
			logger := logging.FromContext(ctx)

			gen := &keyGenerator{}

			// batch time, one hour after midnight.
			// Will be used as the offset when creating TEKs.
			batchTime := timeutils.UTCMidnight(time.Now().UTC()).Add(time.Hour)

			sameDay := timeutils.UTCMidnight(batchTime)

			input := []*exportproto.TemporaryExposureKey{
				{ // Valid key, fully expired, missing report type and transmission risk
					KeyData:                    gen.fakeExposureKey(t),
					RollingStartIntervalNumber: proto.Int32(pubmodel.IntervalNumber(sameDay.Add(-24 * time.Hour))),
					RollingPeriod:              proto.Int32(144),
				},
				{ // same day key, should be future dated
					KeyData:                    gen.fakeExposureKey(t),
					RollingStartIntervalNumber: proto.Int32(pubmodel.IntervalNumber(sameDay)),
					RollingPeriod:              proto.Int32(144),
				},
			}

			appPackageName := "import-export-test"
			regions := []string{"US"}
			settings := &transformer{
				appPackageName: appPackageName,
				importRegions:  regions,
				traveler:       tc.traveler,
				batchTime:      batchTime,
				truncateWindow: time.Hour,
				exportImportID: 7,
				importFileID:   42,
				exportImportConfig: &pubmodel.ExportImportConfig{
					DefaultReportType:         verifyapi.ReportTypeConfirmed,
					BackfillSymptomOnset:      true,
					BackfillSymptomOnsetValue: 10,
					MaxSymptomOnsetDays:       28,
					AllowClinical:             false,
					AllowRevoked:              false,
				},
				logger: logger,
			}

			got, _ := settings.transform(input)

			want := []*pubmodel.Exposure{
				{
					ExposureKey:           input[0].KeyData,
					TransmissionRisk:      2,
					AppPackageName:        appPackageName,
					Regions:               regions,
					Traveler:              tc.traveler,
					IntervalNumber:        input[0].GetRollingStartIntervalNumber(),
					IntervalCount:         input[0].GetRollingPeriod(),
					CreatedAt:             batchTime,
					ExportImportID:        &settings.exportImportID,
					ImportFileID:          &settings.importFileID,
					ReportType:            verifyapi.ReportTypeConfirmed,
					DaysSinceSymptomOnset: &settings.exportImportConfig.BackfillSymptomOnsetValue,
				},
				{ // Key hasn't yet expired, so it is future dated into tomorrow.
					ExposureKey:           input[1].KeyData,
					TransmissionRisk:      2,
					AppPackageName:        appPackageName,
					Regions:               regions,
					Traveler:              tc.traveler,
					IntervalNumber:        input[1].GetRollingStartIntervalNumber(),
					IntervalCount:         input[1].GetRollingPeriod(),
					CreatedAt:             timeutils.UTCMidnight(batchTime.Add(24 * time.Hour)).Add(settings.truncateWindow),
					ExportImportID:        &settings.exportImportID,
					ImportFileID:          &settings.importFileID,
					ReportType:            verifyapi.ReportTypeConfirmed,
					DaysSinceSymptomOnset: &settings.exportImportConfig.BackfillSymptomOnsetValue,
				},
			}

			if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(pubmodel.Exposure{})); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}
