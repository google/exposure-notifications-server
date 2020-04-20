package model

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func intervalStart(t time.Time) int64 {
	tenMin, _ := time.ParseDuration("10m")
	return t.Truncate(tenMin).Unix() / int64(tenMin.Seconds())
}

func TestInvalidBase64(t *testing.T) {
	source := &Publish{
		Keys: []DiagnosisKey{
			DiagnosisKey{
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
	intervalStart := intervalStart(time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC))
	source := &Publish{
		Keys: []DiagnosisKey{
			DiagnosisKey{
				Key:           base64.StdEncoding.EncodeToString([]byte("ABC")),
				IntervalStart: intervalStart,
				IntervalCount: 0,
			},
			DiagnosisKey{
				Key:           base64.StdEncoding.EncodeToString([]byte("DEF")),
				IntervalStart: intervalStart + maxIntervalCount,
			},
			DiagnosisKey{
				Key:           base64.StdEncoding.EncodeToString([]byte("123")),
				IntervalStart: intervalStart + 2*maxIntervalCount,
				IntervalCount: maxIntervalCount * 10, // Invalid, should get rounded down
			},
			DiagnosisKey{
				Key:           base64.StdEncoding.EncodeToString([]byte("456")),
				IntervalStart: intervalStart + 3*maxIntervalCount,
				IntervalCount: 42,
			},
		},
		Regions:         []string{"us", "cA", "Mx"}, // will be upcased
		AppPackageName:  "com.google",
		DiagnosisStatus: 2,
		// Verification doesn't matter for transforming.
	}

	want := []Infection{
		Infection{
			DiagnosisKey:  []byte("ABC"),
			IntervalStart: intervalStart,
			IntervalCount: maxIntervalCount,
		},
		Infection{
			DiagnosisKey:  []byte("DEF"),
			IntervalStart: intervalStart + maxIntervalCount,
			IntervalCount: maxIntervalCount,
		},
		Infection{
			DiagnosisKey:  []byte("123"),
			IntervalStart: intervalStart + 2*maxIntervalCount,
			IntervalCount: maxIntervalCount,
		},
		Infection{
			DiagnosisKey:  []byte("456"),
			IntervalStart: intervalStart + 3*maxIntervalCount,
			IntervalCount: 42,
		},
	}
	batchTime := time.Date(2020, 3, 1, 10, 43, 1, 0, time.UTC)
	batchTimeRounded := time.Date(2020, 3, 1, 10, 30, 0, 0, time.UTC)
	for i, v := range want {
		want[i] = Infection{
			DiagnosisKey:    v.DiagnosisKey,
			DiagnosisStatus: 2,
			AppPackageName:  "com.google",
			Regions:         []string{"US", "CA", "MX"},
			IntervalStart:   v.IntervalStart,
			IntervalCount:   v.IntervalCount,
			CreatedAt:       batchTimeRounded,
			LocalProvenance: true,
		}
	}

	got, err := TransformPublish(source, batchTime)
	if err != nil {
		t.Fatalf("TransformPublish returned unexpected error: %v", err)
	}

	opts := cmpopts.IgnoreFields(Infection{}, "K")
	for i := range want {
		if diff := cmp.Diff(want[i], got[i], opts); diff != "" {
			t.Errorf("TransformPublish mismatch (-want +got):\n%v", diff)
		}
	}
}
