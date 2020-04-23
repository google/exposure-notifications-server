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
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func IntervalNumber(t time.Time) int32 {
	tenMin, _ := time.ParseDuration("10m")
	return int32(t.Truncate(tenMin).Unix()) / int32(tenMin.Seconds())
}

func TestInvalidBase64(t *testing.T) {
	source := &Publish{
		Keys: []ExposureKey{
			{
				Key: base64.StdEncoding.EncodeToString([]byte("ABC")) + `2`,
			},
		},
		Regions:         []string{"US"},
		AppPackageName:  "com.google",
		DiagnosisStatus: 0,
		// Verification doesn't matter for transforming.
	}
	batchTime := time.Date(2020, 3, 1, 10, 43, 1, 0, time.UTC)

	_, err := TransformPublish(source, batchTime)
	expErr := `illegal base64 data at input byte 4`
	if err == nil || err.Error() != expErr {
		t.Errorf("expected error '%v', got: %v", expErr, err)
	}
}

func TestTransform(t *testing.T) {
	intervalNumber := IntervalNumber(time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC))
	source := &Publish{
		Keys: []ExposureKey{
			{
				Key:            base64.StdEncoding.EncodeToString([]byte("ABC")),
				IntervalNumber: intervalNumber,
				IntervalCount:  0,
			},
			{
				Key:            base64.StdEncoding.EncodeToString([]byte("DEF")),
				IntervalNumber: intervalNumber + maxIntervalCount,
			},
			{
				Key:            base64.StdEncoding.EncodeToString([]byte("123")),
				IntervalNumber: intervalNumber + 2*maxIntervalCount,
				IntervalCount:  maxIntervalCount * 10, // Invalid, should get rounded down
			},
			{
				Key:            base64.StdEncoding.EncodeToString([]byte("456")),
				IntervalNumber: intervalNumber + 3*maxIntervalCount,
				IntervalCount:  42,
			},
		},
		Regions:         []string{"us", "cA", "Mx"}, // will be upcased
		AppPackageName:  "com.google",
		DiagnosisStatus: 2,
		// Verification doesn't matter for transforming.
	}

	want := []*Infection{
		{
			ExposureKey:    []byte("ABC"),
			IntervalNumber: intervalNumber,
			IntervalCount:  maxIntervalCount,
		},
		{
			ExposureKey:    []byte("DEF"),
			IntervalNumber: intervalNumber + maxIntervalCount,
			IntervalCount:  maxIntervalCount,
		},
		{
			ExposureKey:    []byte("123"),
			IntervalNumber: intervalNumber + 2*maxIntervalCount,
			IntervalCount:  maxIntervalCount,
		},
		{
			ExposureKey:    []byte("456"),
			IntervalNumber: intervalNumber + 3*maxIntervalCount,
			IntervalCount:  42,
		},
	}
	batchTime := time.Date(2020, 3, 1, 10, 43, 1, 0, time.UTC)
	batchTimeRounded := time.Date(2020, 3, 1, 10, 30, 0, 0, time.UTC)
	for i, v := range want {
		want[i] = &Infection{
			ExposureKey:     v.ExposureKey,
			DiagnosisStatus: 2,
			AppPackageName:  "com.google",
			Regions:         []string{"US", "CA", "MX"},
			IntervalNumber:  v.IntervalNumber,
			IntervalCount:   v.IntervalCount,
			CreatedAt:       batchTimeRounded,
			LocalProvenance: true,
		}
	}

	got, err := TransformPublish(source, batchTime)
	if err != nil {
		t.Fatalf("TransformPublish returned unexpected error: %v", err)
	}

	for i := range want {
		if diff := cmp.Diff(want[i], got[i]); diff != "" {
			t.Errorf("TransformPublish mismatch (-want +got):\n%v", diff)
		}
	}
}
