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

package model

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/verification"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const (
	maxSymptomOnsetDays = 21
)

type testConfig struct {
	maxExposureKeys     uint
	maxSameDayKeys      uint
	maxIntervalStartAge time.Duration
	truncateWindow      time.Duration
	maxSymptomOnsetDays uint
	debugReleaseSameDay bool
}

func (c *testConfig) MaxExposureKeys() uint {
	return c.maxExposureKeys
}

func (c *testConfig) MaxSameDayKeys() uint {
	return c.maxSameDayKeys
}

func (c *testConfig) MaxIntervalStartAge() time.Duration {
	return c.maxIntervalStartAge
}

func (c *testConfig) TruncateWindow() time.Duration {
	return c.truncateWindow
}

func (c *testConfig) MaxSymptomOnsetDays() uint {
	return c.maxSymptomOnsetDays
}

func (c *testConfig) DebugReleaseSameDayKeys() bool {
	return c.debugReleaseSameDay
}

func TestIntervalNumber(t *testing.T) {
	// Since time to interval is lossy, truncate down to the beginnging of a window.
	now := time.Now().Truncate(verifyapi.IntervalLength)

	interval := IntervalNumber(now)
	timeForInterval := TimeForIntervalNumber(interval)

	if now.Unix() != timeForInterval.Unix() {
		t.Errorf("interval mismatch, want: %v got %v", now.Unix(), timeForInterval.Unix())
	}
}

func TestInvalidNew(t *testing.T) {
	errMsg := "maxExposureKeys must be > 0"
	cases := []struct {
		maxKeys        uint
		maxSameDayKeys uint
		message        string
	}{
		{0, 3, "maxExposureKeys must be > 0"},
		{1, 3, ""},
		{5, 1, ""},
		{5, 0, "maxSameDayKeys must be >= 1, got"},
	}

	for i, c := range cases {
		_, err := NewTransformer(&testConfig{c.maxKeys, c.maxSameDayKeys, time.Hour, time.Hour, maxSymptomOnsetDays, false})
		if err != nil && errMsg == "" {
			t.Errorf("%v unexpected error: %v", i, err)
		} else if err != nil && !strings.Contains(err.Error(), c.message) {
			t.Errorf("%v error want '%v', got '%v'", i, c.message, err)
		}
	}
}

func TestInvalidBase64(t *testing.T) {
	ctx := context.Background()
	transformer, err := NewTransformer(&testConfig{1, 1, time.Hour * 24, time.Hour, maxSymptomOnsetDays, false})
	if err != nil {
		t.Fatalf("error creating transformer: %v", err)
	}
	source := &verifyapi.Publish{
		Keys: []verifyapi.ExposureKey{
			{
				Key: base64.StdEncoding.EncodeToString([]byte("ABC")) + `2`,
			},
		},
		HealthAuthorityID: "State Health Dept",
		// Verification doesn't matter for transforming.
	}
	regions := []string{"US"}
	batchTime := time.Date(2020, 3, 1, 10, 43, 1, 0, time.UTC)

	_, err = transformer.TransformPublish(ctx, source, regions, nil, batchTime)
	expErr := `key 0 cannot be imported: illegal base64 data at input byte 4`
	if err == nil || !strings.Contains(err.Error(), expErr) {
		t.Errorf("expected error '%v', got: %v", expErr, err)
	}
}

func TestDifferentEncodings(t *testing.T) {
	data := "this is some data"

	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "std encoding",
			input: base64.StdEncoding.EncodeToString([]byte(data)),
		},
		{
			name:  "raw std encoding",
			input: base64.RawStdEncoding.EncodeToString([]byte(data)),
		},
	}

	for _, c := range cases {
		decoded, err := base64util.DecodeString(c.input)
		if err != nil {
			t.Errorf("%v error: %v", c.name, err)
		} else if string(decoded) != data {
			t.Errorf("%v: want %v got %v", c.name, data, decoded)
		}
	}
}

func TestPublishValidation(t *testing.T) {
	maxAge := 24 * 5 * time.Hour

	captureStartTime := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	currentInterval := IntervalNumber(captureStartTime)
	minInterval := IntervalNumber(captureStartTime.Add(-1 * maxAge))

	cases := []struct {
		name    string
		p       *verifyapi.Publish
		m       string
		sameDay bool
	}{
		{
			name: "no keys",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{},
			},
			m: "no exposure keys in publish request",
		},
		{
			name: "too many exposure keys",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{Key: "foo"},
					{Key: "bar"},
					{Key: "baz"},
				},
			},
			m: "too many exposure keys in publish: 3, max of 2",
		},
		{
			name: "transmission risk too low",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   currentInterval - 2,
						IntervalCount:    1,
						TransmissionRisk: verifyapi.MinTransmissionRisk - 1,
					},
				},
			},
			m: fmt.Sprintf("invalid transmission risk: %v, must be >= %v && <= %v", verifyapi.MinTransmissionRisk-1, verifyapi.MinTransmissionRisk, verifyapi.MaxTransmissionRisk),
		},
		{
			name: "tranismission risk too high",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   currentInterval - 2,
						IntervalCount:    1,
						TransmissionRisk: verifyapi.MaxTransmissionRisk + 1,
					},
				},
			},
			m: fmt.Sprintf("invalid transmission risk: %v, must be >= %v && <= %v", verifyapi.MaxTransmissionRisk+1, verifyapi.MinTransmissionRisk, verifyapi.MaxTransmissionRisk),
		},
		{
			name: "key length too short",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{Key: encodeKey(generateKey(t)[0 : verifyapi.KeyLength-2])},
				},
			},
			m: fmt.Sprintf("invalid key length, %v, must be %v", verifyapi.KeyLength-2, verifyapi.KeyLength),
		},
		{
			name: "interval count too small",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:           encodeKey(generateKey(t)),
						IntervalCount: verifyapi.MinIntervalCount - 1,
					},
				},
			},
			m: fmt.Sprintf("invalid interval count, %v, must be >= %v && <= %v", verifyapi.MinIntervalCount-1, verifyapi.MinIntervalCount, verifyapi.MaxIntervalCount),
		},
		{
			name: "interval_count_too_high",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:           encodeKey(generateKey(t)),
						IntervalCount: verifyapi.MaxIntervalCount + 1,
					},
				},
			},
			m: fmt.Sprintf("invalid interval count, %v, must be >= %v && <= %v", verifyapi.MaxIntervalCount+1, verifyapi.MinIntervalCount, verifyapi.MaxIntervalCount),
		},
		{
			name: "interval_starts_too_old_but_still_valid_at_min",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: minInterval - 1,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
			},
		},
		{
			name: "key_expires_before_min",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: minInterval - verifyapi.MaxIntervalCount - 1,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
			},
			m: fmt.Sprintf("key expires before minimum window; %v + %v = %v which is too old, must be >= %v",
				minInterval-verifyapi.MaxIntervalCount-1,
				verifyapi.MaxIntervalCount,
				minInterval-verifyapi.MaxIntervalCount-1+verifyapi.MaxIntervalCount,
				minInterval),
		},
		{
			name: "interval number too high",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: currentInterval + 1,
						IntervalCount:  1,
					},
				},
			},
			m: fmt.Sprintf("interval number %v is in the future, must be <= %v", currentInterval+1, currentInterval),
		},
		{
			name: "DEBUG: allow end of current UTC day still valid",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: IntervalNumber(captureStartTime.UTC().Truncate(24 * time.Hour)),
						IntervalCount:  144,
					},
				},
			},
			sameDay: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			tf, err := NewTransformer(&testConfig{2, 1, maxAge, time.Hour, maxSymptomOnsetDays, c.sameDay})
			if err != nil {
				t.Fatalf("unepected error: %v", err)
			}

			_, err = tf.TransformPublish(ctx, c.p, []string{}, nil, captureStartTime)
			if err == nil {
				if c.m != "" {
					t.Errorf("want error '%v', got nil", c.m)
				}
			} else if !strings.Contains(err.Error(), c.m) {
				t.Errorf("want error '%v', got '%v'", c.m, err)
			} else if err != nil && c.m == "" {
				t.Errorf("want error nil, got '%v'", err)
			}
		})
	}
}

func generateKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 16)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("unable to generate random key: %v", err)
	}
	return key
}

func encodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func TestStillValidKey(t *testing.T) {
	now := time.Now()
	batchWindow := TruncateWindow(now, time.Minute)
	intervalNumber := IntervalNumber(now) - 1

	cases := []struct {
		name               string
		source             verifyapi.Publish
		createdAt          time.Time
		releaseSameDayKeys bool
	}{
		{
			name: "release same day keys",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   intervalNumber,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 1,
					},
				},
			},
			createdAt:          batchWindow,
			releaseSameDayKeys: true,
		},
		{
			name: "proper embargo",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   intervalNumber,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 1,
					},
				},
			},
			createdAt:          TruncateWindow(TimeForIntervalNumber(intervalNumber+verifyapi.MaxIntervalCount), time.Minute),
			releaseSameDayKeys: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allowedAge := 2 * 24 * time.Hour
			transformer, err := NewTransformer(&testConfig{10, 1, allowedAge, time.Minute, maxSymptomOnsetDays, tc.releaseSameDayKeys})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			tf, err := transformer.TransformPublish(ctx, &tc.source, []string{}, nil, now)
			if err != nil {
				t.Fatal(err)
			}

			if len(tf) != 1 {
				t.Fatalf("wrong number of keys, want: 1 got :%v", len(tf))
			}

			if !tf[0].CreatedAt.Equal(tc.createdAt) {
				t.Errorf("wrong createdAt time, want: %v got: %v", tc.createdAt, tf[0].CreatedAt)
			}
		})
	}
}

func TestReportTypeToTransmissionRisk(t *testing.T) {
	cases := []struct {
		name   string
		report string
		inTR   int
		wantTR int
	}{
		{"provided_tr_with_report", verifyapi.ReportTypeClinical, 8, 8},
		{"provided_tr_no_report", "", 7, 7},
		{"positive_report_backfill", verifyapi.ReportTypeConfirmed, 0, verifyapi.TransmissionRiskConfirmedStandard},
		{"clinical_report_backfill", verifyapi.ReportTypeClinical, 0, verifyapi.TransmissionRiskClinical},
		{"negative_report_backfill", verifyapi.ReportTypeNegative, 0, verifyapi.TransmissionRiskNegative},
		{"no_tr_no_report", "", 0, verifyapi.TransmissionRiskUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReportTypeTransmissionRisk(tc.report, tc.inTR)
			if tc.wantTR != got {
				t.Fatalf("wrong output transmission risk, want: %v got %v", tc.wantTR, got)
			}
		})
	}
}

func intPtr(v int) *int              { return &v }
func int32Ptr(v int32) *int32        { return &v }
func int64Ptr(v int64) *int64        { return &v }
func timePtr(t time.Time) *time.Time { return &t }
func stringPtr(s string) *string     { return &s }

func TestTransform(t *testing.T) {
	captureStartTime := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	intervalNumber := IntervalNumber(captureStartTime)

	testKeys := make([][]byte, 15)
	for i := 0; i < len(testKeys); i++ {
		testKeys[i] = generateKey(t)
	}

	const appPackage = "State Health Dept"
	wantRegions := []string{"US", "CA", "MX"}
	batchTime := captureStartTime.Add(time.Hour * 24 * 7)
	batchTimeRounded := TruncateWindow(batchTime, time.Hour)

	cases := []struct {
		Name    string
		Publish *verifyapi.Publish
		Regions []string
		Claims  *verification.VerifiedClaims
		Want    []*Exposure
	}{
		{
			Name: "basic_v1_publish",
			Publish: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(testKeys[0]),
						IntervalNumber:   intervalNumber,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(testKeys[1]),
						IntervalNumber:   intervalNumber + verifyapi.MaxIntervalCount,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 2,
					},
					{
						Key:              encodeKey(testKeys[2]),
						IntervalNumber:   intervalNumber + 2*verifyapi.MaxIntervalCount,
						IntervalCount:    verifyapi.MaxIntervalCount, // Invalid, should get rounded down
						TransmissionRisk: 3,
					},
					{
						Key:              encodeKey(testKeys[3]),
						IntervalNumber:   intervalNumber + 3*verifyapi.MaxIntervalCount,
						IntervalCount:    42,
						TransmissionRisk: 4,
					},
				},
				HealthAuthorityID: appPackage,
			},
			Regions: []string{"us", "cA", "Mx"}, // will be upcased
			Claims:  nil,
			Want: []*Exposure{
				{
					ExposureKey:      testKeys[0],
					IntervalNumber:   intervalNumber,
					IntervalCount:    verifyapi.MaxIntervalCount,
					TransmissionRisk: 1,
					AppPackageName:   appPackage,
					Regions:          wantRegions,
					CreatedAt:        batchTimeRounded,
					LocalProvenance:  true,
				},
				{
					ExposureKey:      testKeys[1],
					IntervalNumber:   intervalNumber + verifyapi.MaxIntervalCount,
					IntervalCount:    verifyapi.MaxIntervalCount,
					TransmissionRisk: 2,
					AppPackageName:   appPackage,
					Regions:          wantRegions,
					CreatedAt:        batchTimeRounded,
					LocalProvenance:  true,
				},
				{
					ExposureKey:      testKeys[2],
					IntervalNumber:   intervalNumber + 2*verifyapi.MaxIntervalCount,
					IntervalCount:    verifyapi.MaxIntervalCount,
					TransmissionRisk: 3,
					AppPackageName:   appPackage,
					Regions:          wantRegions,
					CreatedAt:        batchTimeRounded,
					LocalProvenance:  true,
				},
				{
					ExposureKey:      testKeys[3],
					IntervalNumber:   intervalNumber + 3*verifyapi.MaxIntervalCount,
					IntervalCount:    42,
					TransmissionRisk: 4,
					AppPackageName:   appPackage,
					Regions:          wantRegions,
					CreatedAt:        batchTimeRounded,
					LocalProvenance:  true,
				},
			},
		},
		{
			Name: "report_type_transmission_risks",
			Publish: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(testKeys[0]),
						IntervalNumber:   intervalNumber,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 0,
					},
					{
						Key:              encodeKey(testKeys[1]),
						IntervalNumber:   intervalNumber + verifyapi.MaxIntervalCount,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 0,
					},
				},
				HealthAuthorityID: appPackage,
			},
			Regions: wantRegions,
			Claims: &verification.VerifiedClaims{
				ReportType: verifyapi.ReportTypeConfirmed,
			},
			Want: []*Exposure{
				{
					ExposureKey:      testKeys[0],
					IntervalNumber:   intervalNumber,
					IntervalCount:    verifyapi.MaxIntervalCount,
					TransmissionRisk: verifyapi.TransmissionRiskConfirmedStandard,
					AppPackageName:   appPackage,
					Regions:          wantRegions,
					CreatedAt:        batchTimeRounded,
					LocalProvenance:  true,
					ReportType:       verifyapi.ReportTypeConfirmed,
				},
				{
					ExposureKey:      testKeys[1],
					IntervalNumber:   intervalNumber + verifyapi.MaxIntervalCount,
					IntervalCount:    verifyapi.MaxIntervalCount,
					TransmissionRisk: verifyapi.TransmissionRiskConfirmedStandard,
					AppPackageName:   appPackage,
					Regions:          wantRegions,
					CreatedAt:        batchTimeRounded,
					LocalProvenance:  true,
					ReportType:       verifyapi.ReportTypeConfirmed,
				},
			},
		},
		{
			Name: "claims_with_report_type_no_backfill",
			Publish: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(testKeys[3]),
						IntervalNumber:   intervalNumber,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 7,
					},
					{
						Key:              encodeKey(testKeys[4]),
						IntervalNumber:   intervalNumber + verifyapi.MaxIntervalCount,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 7,
					},
					{
						Key:              encodeKey(testKeys[5]),
						IntervalNumber:   intervalNumber + 2*verifyapi.MaxIntervalCount,
						IntervalCount:    verifyapi.MaxIntervalCount,
						TransmissionRisk: 7,
					},
				},
				HealthAuthorityID: appPackage,
			},
			Regions: wantRegions,
			Claims: &verification.VerifiedClaims{
				ReportType:           verifyapi.ReportTypeConfirmed,
				SymptomOnsetInterval: uint32(intervalNumber + verifyapi.MaxIntervalCount),
			},
			Want: []*Exposure{
				{
					ExposureKey:           testKeys[3],
					IntervalNumber:        intervalNumber,
					IntervalCount:         verifyapi.MaxIntervalCount,
					TransmissionRisk:      7, // was provided, shouldn't be changed
					AppPackageName:        appPackage,
					Regions:               wantRegions,
					CreatedAt:             batchTimeRounded,
					LocalProvenance:       true,
					ReportType:            verifyapi.ReportTypeConfirmed,
					DaysSinceSymptomOnset: int32Ptr(-1),
				},
				{
					ExposureKey:           testKeys[4],
					IntervalNumber:        intervalNumber + verifyapi.MaxIntervalCount,
					IntervalCount:         verifyapi.MaxIntervalCount,
					TransmissionRisk:      7, // was provided, shouldn't be changed
					AppPackageName:        appPackage,
					Regions:               wantRegions,
					CreatedAt:             batchTimeRounded,
					LocalProvenance:       true,
					ReportType:            verifyapi.ReportTypeConfirmed,
					DaysSinceSymptomOnset: int32Ptr(0),
				},
				{
					ExposureKey:           testKeys[5],
					IntervalNumber:        intervalNumber + 2*verifyapi.MaxIntervalCount,
					IntervalCount:         verifyapi.MaxIntervalCount,
					TransmissionRisk:      7, // was provided, shouldn't be changed
					AppPackageName:        appPackage,
					Regions:               wantRegions,
					CreatedAt:             batchTimeRounded,
					LocalProvenance:       true,
					ReportType:            verifyapi.ReportTypeConfirmed,
					DaysSinceSymptomOnset: int32Ptr(1),
				},
			},
		},
		{
			Name: "claims_with_report_type_with_backfill",
			Publish: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(testKeys[3]),
						IntervalNumber: intervalNumber,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
					{
						Key:            encodeKey(testKeys[4]),
						IntervalNumber: intervalNumber + verifyapi.MaxIntervalCount,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
					{
						Key:            encodeKey(testKeys[5]),
						IntervalNumber: intervalNumber + 2*verifyapi.MaxIntervalCount,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
				HealthAuthorityID: appPackage,
			},
			Regions: wantRegions,
			Claims: &verification.VerifiedClaims{
				HealthAuthorityID:    27,
				ReportType:           verifyapi.ReportTypeClinical,
				SymptomOnsetInterval: uint32(intervalNumber + 2*verifyapi.MaxIntervalCount),
			},
			Want: []*Exposure{
				{
					ExposureKey:           testKeys[3],
					IntervalNumber:        intervalNumber,
					IntervalCount:         verifyapi.MaxIntervalCount,
					TransmissionRisk:      verifyapi.TransmissionRiskClinical,
					AppPackageName:        appPackage,
					Regions:               wantRegions,
					CreatedAt:             batchTimeRounded,
					LocalProvenance:       true,
					ReportType:            verifyapi.ReportTypeClinical,
					DaysSinceSymptomOnset: int32Ptr(-2),
					HealthAuthorityID:     int64Ptr(27),
				},
				{
					ExposureKey:           testKeys[4],
					IntervalNumber:        intervalNumber + verifyapi.MaxIntervalCount,
					IntervalCount:         verifyapi.MaxIntervalCount,
					TransmissionRisk:      verifyapi.TransmissionRiskClinical,
					AppPackageName:        appPackage,
					Regions:               wantRegions,
					CreatedAt:             batchTimeRounded,
					LocalProvenance:       true,
					ReportType:            verifyapi.ReportTypeClinical,
					DaysSinceSymptomOnset: int32Ptr(-1),
					HealthAuthorityID:     int64Ptr(27),
				},
				{
					ExposureKey:           testKeys[5],
					IntervalNumber:        intervalNumber + 2*verifyapi.MaxIntervalCount,
					IntervalCount:         verifyapi.MaxIntervalCount,
					TransmissionRisk:      verifyapi.TransmissionRiskClinical,
					AppPackageName:        appPackage,
					Regions:               wantRegions,
					CreatedAt:             batchTimeRounded,
					LocalProvenance:       true,
					ReportType:            verifyapi.ReportTypeClinical,
					DaysSinceSymptomOnset: int32Ptr(0),
					HealthAuthorityID:     int64Ptr(27),
				},
			},
		},
	}

	allowedAge := 14 * 24 * time.Hour
	transformer, err := NewTransformer(&testConfig{10, 1, allowedAge, time.Hour, maxSymptomOnsetDays, false})
	if err != nil {
		t.Fatalf("NewTransformer returned unexpected error: %v", err)
	}
	ctx := context.Background()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := transformer.TransformPublish(ctx, tc.Publish, tc.Regions, tc.Claims, batchTime)
			if err != nil {
				t.Fatalf("TransformPublish returned unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.Want, got, cmpopts.IgnoreUnexported(Exposure{})); diff != "" {
				t.Errorf("TransformPublish mismatch (-want +got):\n%v", diff)
			}
		})
	}
}

func TestTransformOverlapping(t *testing.T) {
	now := time.Now()
	allowedAge := 3 * 24 * time.Hour
	twoDaysAgoInterval := IntervalNumber(now) - 1 - 288
	oneDayAgoInterval := IntervalNumber(now) - 1 - 144

	cases := []struct {
		name                string
		source              verifyapi.Publish
		regions             []string
		maxSameIntervalKeys uint
		error               string
	}{
		{
			name: "invalid_overlap_in_order",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: twoDaysAgoInterval,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: twoDaysAgoInterval + verifyapi.MaxIntervalCount - 2,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
				HealthAuthorityID: "State Health Dept",
			},
			regions:             []string{"us", "cA", "Mx"}, // will be upcased
			maxSameIntervalKeys: 3,
			error:               "exposure keys have non aligned overlapping intervals",
		},
		{
			name: "invalid_overlap_out_of_order",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: twoDaysAgoInterval,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: twoDaysAgoInterval - verifyapi.MaxIntervalCount + 1,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
				HealthAuthorityID: "State Health Dept",
			},
			regions:             []string{"us", "cA", "Mx"}, // will be upcased
			maxSameIntervalKeys: 3,
			error:               "exposure keys have non aligned overlapping intervals",
		},
		{
			name: "allowed_number_of_same_day_keys",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   twoDaysAgoInterval,
						IntervalCount:    44,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   twoDaysAgoInterval,
						IntervalCount:    88,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   twoDaysAgoInterval,
						IntervalCount:    144,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   oneDayAgoInterval,
						IntervalCount:    44,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   oneDayAgoInterval,
						IntervalCount:    88,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   oneDayAgoInterval,
						IntervalCount:    144,
						TransmissionRisk: 1,
					},
				},
			},
			regions:             []string{"US"},
			maxSameIntervalKeys: 3,
			error:               "",
		},
		{
			name: "too_many_same_day_keys",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   twoDaysAgoInterval,
						IntervalCount:    44,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   twoDaysAgoInterval,
						IntervalCount:    88,
						TransmissionRisk: 1,
					},
					{
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   twoDaysAgoInterval,
						IntervalCount:    144,
						TransmissionRisk: 1,
					},
					{
						// Out of order - these will be sorted.
						Key:              encodeKey(generateKey(t)),
						IntervalNumber:   twoDaysAgoInterval,
						IntervalCount:    88,
						TransmissionRisk: 1,
					},
				},
			},
			regions:             []string{"US"},
			maxSameIntervalKeys: 3,
			error:               fmt.Sprintf("too many overlapping keys for start interval: %v want: <= 3, got: 4", twoDaysAgoInterval),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			transformer, err := NewTransformer(&testConfig{10, tc.maxSameIntervalKeys, allowedAge, time.Hour, maxSymptomOnsetDays, false})
			if err != nil {
				t.Fatalf("NewTransformer returned unexpected error: %v", err)
			}
			_, err = transformer.TransformPublish(ctx, &tc.source, tc.regions, nil, now)
			if err != nil && tc.error == "" {
				t.Fatalf("unexpected error, want: nil, got: %v", err)
			} else if err != nil && !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("wrong error: want '%v', got: %v", tc.error, err.Error())
			} else if err == nil && tc.error != "" {
				t.Fatalf("missing error: want '%v', got: nil", tc.error)
			}
		})
	}
}

func TestDaysFromSymptomOnset(t *testing.T) {
	now := time.Now().UTC()

	cases := []struct {
		name  string
		onset int32
		check int32
		want  int32
	}{
		{
			name:  "exact_match",
			onset: IntervalNumber(now),
			check: IntervalNumber(now),
			want:  0,
		},
		{
			name:  "next_day",
			onset: IntervalNumber(now),
			check: IntervalNumber(now.Add(24 * time.Hour)),
			want:  1,
		},
		{
			name:  "next_day_round_down",
			onset: IntervalNumber(now),
			check: IntervalNumber(now.Add(35 * time.Hour)),
			want:  1,
		},
		{
			name:  "next_day_round_up",
			onset: IntervalNumber(now),
			check: IntervalNumber(now.Add(37 * time.Hour)),
			want:  2,
		},
		{
			name:  "previous_day",
			onset: IntervalNumber(now),
			check: IntervalNumber(now.Add(-24 * time.Hour)),
			want:  -1,
		},
		{
			name:  "previous_day_round_down",
			onset: IntervalNumber(now),
			check: IntervalNumber(now.Add(-25 * time.Hour)),
			want:  -1,
		},
		{
			name:  "previous_day_round_up",
			onset: IntervalNumber(now),
			check: IntervalNumber(now.Add(-47 * time.Hour)),
			want:  -2,
		},
		{
			name:  "multiple_days",
			onset: IntervalNumber(now),
			check: IntervalNumber(now.Add(8*24*time.Hour + 2*time.Hour)),
			want:  8,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DaysFromSymptomOnset(tc.onset, tc.check)
			if tc.want != got {
				t.Fatalf("wrong day instance between %v and %v, got: %v want: %v", tc.onset, tc.check, got, tc.want)
			}
		})
	}
}

func TestReviseKeys(t *testing.T) {
	createdAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour)
	revisedAt := time.Now().UTC().Truncate(time.Hour)

	allExposures := make([]*Exposure, 4)
	// The "existing" key that isn't in the revision set.
	allExposures[0] = &Exposure{
		ExposureKey: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	}
	// Existing key that is in the revision set
	allExposures[1] = &Exposure{
		ExposureKey:       []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		TransmissionRisk:  0,
		Regions:           []string{"US"},
		IntervalNumber:    7,
		IntervalCount:     144,
		CreatedAt:         createdAt,
		LocalProvenance:   true,
		HealthAuthorityID: int64Ptr(2),
		ReportType:        verifyapi.ReportTypeClinical,
	}
	// New version of existing key
	allExposures[2] = &Exposure{
		ExposureKey:       []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		TransmissionRisk:  2,
		Regions:           []string{"US"},
		IntervalNumber:    7,
		IntervalCount:     144,
		CreatedAt:         revisedAt,
		LocalProvenance:   true,
		HealthAuthorityID: int64Ptr(2),
		ReportType:        verifyapi.ReportTypeConfirmed,
	}
	// New key not in existing set.
	allExposures[3] = &Exposure{
		ExposureKey:       []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		TransmissionRisk:  0,
		Regions:           []string{"US"},
		IntervalNumber:    8,
		IntervalCount:     144,
		CreatedAt:         createdAt,
		LocalProvenance:   true,
		HealthAuthorityID: int64Ptr(2),
		ReportType:        verifyapi.ReportTypeConfirmed,
	}

	ctx := context.Background()
	existing := make(map[string]*Exposure)
	existing[allExposures[0].ExposureKeyBase64()] = allExposures[0]
	existing[allExposures[1].ExposureKeyBase64()] = allExposures[1]

	incoming := make([]*Exposure, 2)
	incoming[0] = allExposures[2]
	incoming[1] = allExposures[3]

	got, err := ReviseKeys(ctx, existing, incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []*Exposure{
		{
			ExposureKey:             []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			TransmissionRisk:        0,
			Regions:                 []string{"US"},
			IntervalNumber:          7,
			IntervalCount:           144,
			CreatedAt:               createdAt,
			LocalProvenance:         true,
			HealthAuthorityID:       int64Ptr(2),
			ReportType:              verifyapi.ReportTypeClinical,
			RevisedAt:               &revisedAt,
			RevisedReportType:       stringPtr(verifyapi.ReportTypeConfirmed),
			RevisedTransmissionRisk: intPtr(2),
		},
		{
			ExposureKey:       []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
			TransmissionRisk:  0,
			Regions:           []string{"US"},
			IntervalNumber:    8,
			IntervalCount:     144,
			CreatedAt:         createdAt,
			LocalProvenance:   true,
			HealthAuthorityID: int64Ptr(2),
			ReportType:        verifyapi.ReportTypeConfirmed,
		},
	}

	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(Exposure{})); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestExposureReview(t *testing.T) {
	createdAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Hour)
	revisedAt := time.Now().UTC().Add(time.Hour).Truncate(time.Hour)

	cases := []struct {
		name          string
		previous      *Exposure
		incoming      *Exposure
		want          *Exposure
		needsRevision bool
		err           string
	}{
		{
			name: "matching_report_type",
			previous: &Exposure{
				ReportType: verifyapi.ReportTypeConfirmed,
			},
			incoming: &Exposure{
				ReportType: verifyapi.ReportTypeConfirmed,
			},
			needsRevision: false,
			err:           "",
		},
		{
			name: "invalid_provenance",
			previous: &Exposure{
				ReportType: verifyapi.ReportTypeClinical,
			},
			incoming: &Exposure{
				ReportType: verifyapi.ReportTypeConfirmed,
			},
			needsRevision: false,
			err:           ErrorNonLocalProvenance.Error(),
		},
		{
			name: "already_revised",
			previous: &Exposure{
				ReportType:      verifyapi.ReportTypeClinical,
				LocalProvenance: true,
				RevisedAt:       timePtr(time.Now().UTC()),
			},
			incoming: &Exposure{
				ReportType: verifyapi.ReportTypeConfirmed,
			},
			needsRevision: false,
			err:           ErrorKeyAlreadyRevised.Error(),
		},
		{
			name: "invalid_transition_confirmed_to_clinical",
			previous: &Exposure{
				ReportType:      verifyapi.ReportTypeConfirmed,
				LocalProvenance: true,
			},
			incoming: &Exposure{
				ReportType: verifyapi.ReportTypeClinical,
			},
			needsRevision: false,
			err:           "invalid report type transition, cannot transition from 'confirmed' to 'likely'",
		},
		{
			name: "invalid_transition_from_empty_report_type",
			previous: &Exposure{
				ReportType:      "",
				LocalProvenance: true,
			},
			incoming: &Exposure{
				ReportType: verifyapi.ReportTypeClinical,
			},
			needsRevision: false,
			err:           "invalid report type transition, cannot transition from '' to 'likely'",
		},
		{
			name: "valid_transition_from_empty_report_type",
			previous: &Exposure{
				ReportType:      "",
				LocalProvenance: true,
			},
			incoming: &Exposure{
				ReportType: verifyapi.ReportTypeConfirmed,
				CreatedAt:  revisedAt,
			},
			want: &Exposure{
				ReportType:              "",
				LocalProvenance:         true,
				RevisedReportType:       stringPtr(verifyapi.ReportTypeConfirmed),
				RevisedAt:               &revisedAt,
				RevisedTransmissionRisk: intPtr(verifyapi.TransmissionRiskConfirmedStandard),
			},
			needsRevision: true,
		},
		{
			name: "revise_key",
			previous: &Exposure{
				ReportType:            verifyapi.ReportTypeClinical,
				LocalProvenance:       true,
				HealthAuthorityID:     int64Ptr(2),
				Regions:               []string{"US", "CA"},
				TransmissionRisk:      4,
				CreatedAt:             createdAt,
				DaysSinceSymptomOnset: int32Ptr(-1),
			},
			incoming: &Exposure{
				ReportType:            verifyapi.ReportTypeConfirmed,
				HealthAuthorityID:     int64Ptr(3),
				Regions:               []string{"MX"},
				TransmissionRisk:      5,
				CreatedAt:             revisedAt,
				DaysSinceSymptomOnset: int32Ptr(0),
			},
			want: &Exposure{
				ReportType:                   verifyapi.ReportTypeClinical,
				LocalProvenance:              true,
				HealthAuthorityID:            int64Ptr(3),
				Regions:                      []string{"US", "CA", "MX"},
				TransmissionRisk:             4,
				CreatedAt:                    createdAt,
				DaysSinceSymptomOnset:        int32Ptr(-1),
				RevisedReportType:            stringPtr(verifyapi.ReportTypeConfirmed),
				RevisedAt:                    &revisedAt,
				RevisedDaysSinceSymptomOnset: int32Ptr(0),
				RevisedTransmissionRisk:      intPtr(5),
			},
			needsRevision: true,
			err:           "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.previous.Revise(tc.incoming)
			if result != tc.needsRevision {
				t.Errorf("revision decision mismatch: want: %v got: %v", tc.needsRevision, result)
			}
			if err != nil && tc.err == "" {
				t.Fatalf("unexpected error: %v", err)
			} else if err != nil && !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("wrong error: want '%v', got: '%v'", tc.err, err)
			} else if err == nil && tc.err != "" {
				t.Fatalf("expected error: want '%v', got: nil", tc.err)
			}
			if tc.err != "" || !tc.needsRevision {
				return
			}

			if diff := cmp.Diff(tc.want, tc.previous, cmpopts.IgnoreUnexported(Exposure{})); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
