package model

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestInvalidBase64(t *testing.T) {
	keyDay := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	source := &Publish{
		Keys: []DiagnosisKey{
			DiagnosisKey{
				Key:    base64.StdEncoding.EncodeToString([]byte("ABC")) + `2`,
				KeyDay: keyDay.Unix(),
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
	keyDay := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	oneDay, _ := time.ParseDuration("24h")
	source := &Publish{
		Keys: []DiagnosisKey{
			DiagnosisKey{
				Key:    base64.StdEncoding.EncodeToString([]byte("ABC")),
				KeyDay: keyDay.Unix(),
			},
			DiagnosisKey{
				Key:    base64.StdEncoding.EncodeToString([]byte("DEF")),
				KeyDay: keyDay.Add(oneDay).Unix(),
			},
			DiagnosisKey{
				Key:    base64.StdEncoding.EncodeToString([]byte("123")),
				KeyDay: keyDay.Add(oneDay).Add(oneDay).Unix(),
			},
		},
		Regions:         []string{"us", "cA", "Mx"}, // will be upcased
		AppPackageName:  "com.google",
		DiagnosisStatus: 2,
		// Verification doesn't matter for transforming.
	}

	want := []Infection{
		Infection{DiagnosisKey: []byte("ABC")},
		Infection{DiagnosisKey: []byte("DEF")},
		Infection{DiagnosisKey: []byte("123")},
	}
	keyDayRounded := time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC)
	batchTime := time.Date(2020, 3, 1, 10, 43, 1, 0, time.UTC)
	batchTimeRounded := time.Date(2020, 3, 1, 10, 30, 0, 0, time.UTC)
	for i, v := range want {
		want[i] = Infection{
			DiagnosisKey:     v.DiagnosisKey,
			DiagnosisStatus:  2,
			AppPackageName:   "com.google",
			Regions:          []string{"US", "CA", "MX"},
			FederationSyncId: "0",
			KeyDay:           keyDayRounded,
			KeyDuration:      0,
			CreatedAt:        batchTimeRounded,
			LocalProvenance:  true,
		}
		keyDayRounded = keyDayRounded.Add(oneDay)
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
