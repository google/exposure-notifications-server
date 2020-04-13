package model

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestTransform(t *testing.T) {
	source := &Publish{
		Keys: []string{
			base64.StdEncoding.EncodeToString([]byte("ABC")),
			base64.StdEncoding.EncodeToString([]byte("DEF")),
			base64.StdEncoding.EncodeToString([]byte("123")),
		},
		AppPackageName: "com.google",
		Country:        "us",
		Platform:       "android",
		// Verification doesn't matter for transforming.
	}

	want := []Infection{
		Infection{DiagnosisKey: []byte("ABC")},
		Infection{DiagnosisKey: []byte("DEF")},
		Infection{DiagnosisKey: []byte("123")},
	}
	batchTime := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	for i, v := range want {
		want[i] = Infection{
			DiagnosisKey:     v.DiagnosisKey,
			AppPackageName:   "com.google",
			Country:          "us",
			Platform:         "android",
			FederationSyncId: 0,
			BatchTimestamp:   batchTime,
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
