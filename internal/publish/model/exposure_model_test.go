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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/base64util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
)

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
	errMsg := fmt.Sprintf("maxExposureKeys must be > 0 and <= %v", verifyapi.MaxKeysPerPublish)
	cases := []struct {
		maxKeys int
		message string
	}{
		{0, errMsg},
		{1, ""},
		{5, ""},
		{verifyapi.MaxKeysPerPublish - 1, ""},
		{verifyapi.MaxKeysPerPublish, ""},
		{verifyapi.MaxKeysPerPublish + 1, errMsg},
	}

	for i, c := range cases {
		_, err := NewTransformer(c.maxKeys, time.Hour, time.Hour, false)
		if err != nil && errMsg == "" {
			t.Errorf("%v unexpected error: %v", i, err)
		} else if err != nil && !strings.Contains(err.Error(), c.message) {
			t.Errorf("%v error want '%v', got '%v'", i, c.message, err)
		}
	}
}

func TestInvalidBase64(t *testing.T) {
	ctx := context.Background()
	transformer, err := NewTransformer(1, time.Hour*24, time.Hour, false)
	if err != nil {
		t.Fatalf("error creating transformer: %v", err)
	}
	source := &verifyapi.Publish{
		Keys: []verifyapi.ExposureKey{
			{
				Key: base64.StdEncoding.EncodeToString([]byte("ABC")) + `2`,
			},
		},
		Regions:        []string{"US"},
		AppPackageName: "com.google",
		// Verification doesn't matter for transforming.
	}
	batchTime := time.Date(2020, 3, 1, 10, 43, 1, 0, time.UTC)

	_, err = transformer.TransformPublish(ctx, source, batchTime)
	expErr := `invalid publish data: illegal base64 data at input byte 4`
	if err == nil || err.Error() != expErr {
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
			name: "interval count too high",
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
			name: "interval number too low",
			p: &verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: minInterval - 1,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
			},
			m: fmt.Sprintf("interval number %v is too old, must be >= %v", minInterval-1, minInterval),
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
			tf, err := NewTransformer(2, maxAge, time.Hour, c.sameDay)
			if err != nil {
				t.Fatalf("unepected error: %v", err)
			}

			_, err = tf.TransformPublish(ctx, c.p, captureStartTime)
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

func decodeKey(b64key string, t *testing.T) []byte {
	k, err := base64util.DecodeString(b64key)
	if err != nil {
		t.Fatalf("unable to decode key: %v", err)
	}
	return k
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
			transformer, err := NewTransformer(10, allowedAge, time.Minute, tc.releaseSameDayKeys)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			tf, err := transformer.TransformPublish(ctx, &tc.source, now)
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

func TestTransform(t *testing.T) {
	captureStartTime := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	intervalNumber := IntervalNumber(captureStartTime)

	source := &verifyapi.Publish{
		Keys: []verifyapi.ExposureKey{
			{
				Key:              encodeKey(generateKey(t)),
				IntervalNumber:   intervalNumber,
				IntervalCount:    verifyapi.MaxIntervalCount,
				TransmissionRisk: 1,
			},
			{
				Key:              encodeKey(generateKey(t)),
				IntervalNumber:   intervalNumber + verifyapi.MaxIntervalCount,
				IntervalCount:    verifyapi.MaxIntervalCount,
				TransmissionRisk: 2,
			},
			{
				Key:              encodeKey(generateKey(t)),
				IntervalNumber:   intervalNumber + 2*verifyapi.MaxIntervalCount,
				IntervalCount:    verifyapi.MaxIntervalCount, // Invalid, should get rounded down
				TransmissionRisk: 3,
			},
			{
				Key:              encodeKey(generateKey(t)),
				IntervalNumber:   intervalNumber + 3*verifyapi.MaxIntervalCount,
				IntervalCount:    42,
				TransmissionRisk: 4,
			},
		},
		Regions:        []string{"us", "cA", "Mx"}, // will be upcased
		AppPackageName: "com.google",
		// Verification doesn't matter for transforming.
	}

	want := []*Exposure{
		{
			ExposureKey:      decodeKey(source.Keys[0].Key, t),
			IntervalNumber:   intervalNumber,
			IntervalCount:    verifyapi.MaxIntervalCount,
			TransmissionRisk: 1,
		},
		{
			ExposureKey:      decodeKey(source.Keys[1].Key, t),
			IntervalNumber:   intervalNumber + verifyapi.MaxIntervalCount,
			IntervalCount:    verifyapi.MaxIntervalCount,
			TransmissionRisk: 2,
		},
		{
			ExposureKey:      decodeKey(source.Keys[2].Key, t),
			IntervalNumber:   intervalNumber + 2*verifyapi.MaxIntervalCount,
			IntervalCount:    verifyapi.MaxIntervalCount,
			TransmissionRisk: 3,
		},
		{
			ExposureKey:      decodeKey(source.Keys[3].Key, t),
			IntervalNumber:   intervalNumber + 3*verifyapi.MaxIntervalCount,
			IntervalCount:    42,
			TransmissionRisk: 4,
		},
	}
	batchTime := captureStartTime.Add(time.Hour * 24 * 7)
	batchTimeRounded := TruncateWindow(batchTime, time.Hour)
	for i, v := range want {
		want[i] = &Exposure{
			ExposureKey:      v.ExposureKey,
			TransmissionRisk: i + 1,
			AppPackageName:   "com.google",
			Regions:          []string{"US", "CA", "MX"},
			IntervalNumber:   v.IntervalNumber,
			IntervalCount:    v.IntervalCount,
			CreatedAt:        batchTimeRounded,
			LocalProvenance:  true,
		}
	}

	allowedAge := 14 * 24 * time.Hour
	transformer, err := NewTransformer(10, allowedAge, time.Hour, false)
	if err != nil {
		t.Fatalf("NewTransformer returned unexpected error: %v", err)
	}
	ctx := context.Background()
	got, err := transformer.TransformPublish(ctx, source, batchTime)
	if err != nil {
		t.Fatalf("TransformPublish returned unexpected error: %v", err)
	}

	for i := range want {
		if diff := cmp.Diff(want[i], got[i]); diff != "" {
			t.Errorf("TransformPublish mismatch (-want +got):\n%v", diff)
		}
	}
}

func TestTransformOverlapping(t *testing.T) {
	captureStartTime := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	intervalNumber := IntervalNumber(captureStartTime)

	cases := []struct {
		name   string
		source verifyapi.Publish
	}{
		{
			name: "overlap",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: intervalNumber,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: intervalNumber + verifyapi.MaxIntervalCount - 2,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
				Regions:        []string{"us", "cA", "Mx"}, // will be upcased
				AppPackageName: "com.google",
			},
		},
		{
			name: "overlap 2",
			source: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: intervalNumber,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: intervalNumber - verifyapi.MaxIntervalCount + 1,
						IntervalCount:  verifyapi.MaxIntervalCount,
					},
				},
				Regions:        []string{"us", "cA", "Mx"}, // will be upcased
				AppPackageName: "com.google",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			batchTime := captureStartTime.Add(time.Hour * 24 * 7)
			allowedAge := 14 * 24 * time.Hour
			transformer, err := NewTransformer(10, allowedAge, time.Hour, false)
			if err != nil {
				t.Fatalf("NewTransformer returned unexpected error: %v", err)
			}
			_, err = transformer.TransformPublish(ctx, &c.source, batchTime)
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}
			if err.Error() != "exposure keys have overlapping intervals" {
				t.Errorf("Wrong error, want '%v', got '%v'", "exposure key intervals are not consecutive", err)
			}
		})
	}
}

func TestApplyOverrides(t *testing.T) {
	cases := []struct {
		Name      string
		Publish   verifyapi.Publish
		Overrides verifyapi.TransmissionRiskVector
		Want      []verifyapi.ExposureKey
	}{
		{
			Name: "no overrides",
			Publish: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              "A",
						IntervalNumber:   1,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
				},
			},
			Overrides: make([]verifyapi.TransmissionRiskOverride, 0),
			Want: []verifyapi.ExposureKey{
				{
					Key:              "A",
					IntervalNumber:   1,
					IntervalCount:    2,
					TransmissionRisk: 1,
				},
			},
		},
		{
			Name: "aligned interval transmission risks",
			Publish: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              "A",
						IntervalNumber:   1,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
					{
						Key:              "B",
						IntervalNumber:   3,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
					{
						Key:              "C",
						IntervalNumber:   5,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
				},
			},
			Overrides: []verifyapi.TransmissionRiskOverride{
				{
					SinceRollingPeriod: 5,
					TranismissionRisk:  5,
				},
				{
					SinceRollingPeriod: 3,
					TranismissionRisk:  3,
				},
				{
					SinceRollingPeriod: 0,
					TranismissionRisk:  0,
				},
			},
			Want: []verifyapi.ExposureKey{
				{
					Key:              "A",
					IntervalNumber:   1,
					IntervalCount:    2,
					TransmissionRisk: 0,
				},
				{
					Key:              "B",
					IntervalNumber:   3,
					IntervalCount:    2,
					TransmissionRisk: 3,
				},
				{
					Key:              "C",
					IntervalNumber:   5,
					IntervalCount:    2,
					TransmissionRisk: 5,
				},
			},
		},
		{
			Name: "unaligned, with 0 fallback.",
			Publish: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              "A",
						IntervalNumber:   1,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
					{
						Key:              "B",
						IntervalNumber:   3,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
					{
						Key:              "C",
						IntervalNumber:   5,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
				},
			},
			Overrides: []verifyapi.TransmissionRiskOverride{
				{
					SinceRollingPeriod: 4,
					TranismissionRisk:  5, // anything effective at time >= 4 gets TR 5.
				},
				{
					SinceRollingPeriod: 0,
					TranismissionRisk:  2, // 2 since beginning of time.
				},
			},
			Want: []verifyapi.ExposureKey{
				{
					Key:              "A",
					IntervalNumber:   1,
					IntervalCount:    2,
					TransmissionRisk: 2,
				},
				{
					Key:              "B",
					IntervalNumber:   3,
					IntervalCount:    2,
					TransmissionRisk: 5,
				},
				{
					Key:              "C",
					IntervalNumber:   5,
					IntervalCount:    2,
					TransmissionRisk: 5,
				},
			},
		},
		{
			Name: "overrides run out",
			Publish: verifyapi.Publish{
				Keys: []verifyapi.ExposureKey{
					{
						Key:              "A",
						IntervalNumber:   1,
						IntervalCount:    2,
						TransmissionRisk: 1,
					},
				},
			},
			Overrides: []verifyapi.TransmissionRiskOverride{
				{
					SinceRollingPeriod: 4,
					TranismissionRisk:  5, // anything effective at time >= 4 gets TR 5.
				},
			},
			Want: []verifyapi.ExposureKey{
				{
					Key:              "A",
					IntervalNumber:   1,
					IntervalCount:    2,
					TransmissionRisk: 1,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ApplyTransmissionRiskOverrides(&tc.Publish, tc.Overrides)
			sorter := cmp.Transformer("Sort", func(in []verifyapi.ExposureKey) []verifyapi.ExposureKey {
				out := append([]verifyapi.ExposureKey(nil), in...) // Copy input to avoid mutating it
				sort.Slice(out, func(i int, j int) bool {
					return strings.Compare(out[i].Key, out[j].Key) <= 0
				})
				return out
			})
			if diff := cmp.Diff(tc.Want, tc.Publish.Keys, sorter); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
