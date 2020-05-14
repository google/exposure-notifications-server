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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/base64util"
	"github.com/google/go-cmp/cmp"
)

func TestInvalidNew(t *testing.T) {
	errMsg := fmt.Sprintf("maxExposureKeys must be > 0 and <= %v", maxKeysPerPublish)
	cases := []struct {
		maxKeys int
		message string
	}{
		{0, errMsg},
		{1, ""},
		{5, ""},
		{maxKeysPerPublish - 1, ""},
		{maxKeysPerPublish, ""},
		{maxKeysPerPublish + 1, errMsg},
	}

	for i, c := range cases {
		_, err := NewTransformer(c.maxKeys, time.Hour)
		if err != nil && errMsg == "" {
			t.Errorf("%v unexpected error: %v", i, err)
		} else if err != nil && !strings.Contains(err.Error(), c.message) {
			t.Errorf("%v error want '%v', got '%v'", i, c.message, err)
		}
	}
}

func TestInvalidBase64(t *testing.T) {
	transformer, err := NewTransformer(1, time.Hour*24)
	if err != nil {
		t.Fatalf("error creating transformer: %v", err)
	}
	source := &Publish{
		Keys: []ExposureKey{
			{
				Key: base64.StdEncoding.EncodeToString([]byte("ABC")) + `2`,
			},
		},
		Regions:          []string{"US"},
		AppPackageName:   "com.google",
		TransmissionRisk: 1,
		// Verification doesn't matter for transforming.
	}
	batchTime := time.Date(2020, 3, 1, 10, 43, 1, 0, time.UTC)

	_, err = transformer.TransformPublish(source, batchTime)
	expErr := `illegal base64 data at input byte 4`
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
	currentInteval := IntervalNumber(captureStartTime)
	minInterval := IntervalNumber(captureStartTime.Add(-1 * maxAge))

	tf, err := NewTransformer(2, maxAge)
	if err != nil {
		t.Fatalf("unepected error: %v", err)
	}

	cases := []struct {
		name string
		p    *Publish
		m    string
	}{
		{
			name: "no keys",
			p: &Publish{
				Keys: []ExposureKey{},
			},
			m: "no exposure keys in publish request",
		},
		{
			name: "too many exposure keys",
			p: &Publish{
				Keys: []ExposureKey{
					{Key: "foo"},
					{Key: "bar"},
					{Key: "baz"},
				},
			},
			m: "too many exposure keys in publish: 3, max of 2",
		},
		{
			name: "transmission risk too low",
			p: &Publish{
				Keys:             []ExposureKey{{Key: "invalid"}},
				TransmissionRisk: minTransmissionRisk - 1,
			},
			m: fmt.Sprintf("invalid transmission risk: %v, must be >= %v && <= %v", minTransmissionRisk-1, minTransmissionRisk, maxTransmissionRisk),
		},
		{
			name: "tranismission risk too high",
			p: &Publish{
				Keys:             []ExposureKey{{Key: "invalid"}},
				TransmissionRisk: maxTransmissionRisk + 1,
			},
			m: fmt.Sprintf("invalid transmission risk: %v, must be >= %v && <= %v", maxTransmissionRisk+1, minTransmissionRisk, maxTransmissionRisk),
		},
		{
			name: "key length too short",
			p: &Publish{
				Keys: []ExposureKey{
					{Key: encodeKey(generateKey(t)[0 : keyLength-2])},
				},
				TransmissionRisk: maxTransmissionRisk - 1,
			},
			m: fmt.Sprintf("invalid key length, %v, must be %v", keyLength-2, keyLength),
		},
		{
			name: "interval count too small",
			p: &Publish{
				Keys: []ExposureKey{
					{
						Key:           encodeKey(generateKey(t)),
						IntervalCount: minIntervalCount - 1,
					},
				},
				TransmissionRisk: maxTransmissionRisk - 1,
			},
			m: fmt.Sprintf("invalid interval count, %v, must be >= %v && <= %v", minIntervalCount-1, minIntervalCount, maxIntervalCount),
		},
		{
			name: "interval count too high",
			p: &Publish{
				Keys: []ExposureKey{
					{
						Key:           encodeKey(generateKey(t)),
						IntervalCount: maxIntervalCount + 1,
					},
				},
				TransmissionRisk: maxTransmissionRisk - 1,
			},
			m: fmt.Sprintf("invalid interval count, %v, must be >= %v && <= %v", maxIntervalCount+1, minIntervalCount, maxIntervalCount),
		},
		{
			name: "interval number too low",
			p: &Publish{
				Keys: []ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: minInterval - 1,
						IntervalCount:  maxIntervalCount,
					},
				},
				TransmissionRisk: maxTransmissionRisk - 1,
			},
			m: fmt.Sprintf("interval number %v is too old, must be >= %v", minInterval-1, minInterval),
		},
		{
			name: "interval number too high",
			p: &Publish{
				Keys: []ExposureKey{
					{
						Key:            encodeKey(generateKey(t)),
						IntervalNumber: currentInteval + 1,
						IntervalCount:  1,
					},
				},
				TransmissionRisk: maxTransmissionRisk - 1,
			},
			m: fmt.Sprintf("interval number %v is in the future, must be < %v", currentInteval+1, currentInteval),
		},
	}

	for _, c := range cases {
		_, err = tf.TransformPublish(c.p, captureStartTime)
		if err == nil {
			t.Errorf("test '%v': want error '%v', got nil", c.name, c.m)
		} else if !strings.Contains(err.Error(), c.m) {
			t.Errorf("test '%v': want error '%v', got '%v'", c.name, c.m, err)
		} else if err != nil && c.m == "" {
			t.Errorf("test '%v': want error nil, got '%v'", c.name, err)
		}
	}
}

func generateKey(t *testing.T) []byte {
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

func TestTransform(t *testing.T) {
	captureStartTime := time.Date(2020, 2, 29, 11, 15, 1, 0, time.UTC)
	intervalNumber := IntervalNumber(captureStartTime)
	source := &Publish{
		Keys: []ExposureKey{
			{
				Key:            encodeKey(generateKey(t)),
				IntervalNumber: intervalNumber,
				IntervalCount:  maxIntervalCount,
			},
			{
				Key:            encodeKey(generateKey(t)),
				IntervalNumber: intervalNumber + maxIntervalCount,
				IntervalCount:  maxIntervalCount,
			},
			{
				Key:            encodeKey(generateKey(t)),
				IntervalNumber: intervalNumber + 2*maxIntervalCount,
				IntervalCount:  maxIntervalCount, // Invalid, should get rounded down
			},
			{
				Key:            encodeKey(generateKey(t)),
				IntervalNumber: intervalNumber + 3*maxIntervalCount,
				IntervalCount:  42,
			},
		},
		Regions:          []string{"us", "cA", "Mx"}, // will be upcased
		AppPackageName:   "com.google",
		TransmissionRisk: 2,
		// Verification doesn't matter for transforming.
	}

	want := []*Exposure{
		{
			ExposureKey:    decodeKey(source.Keys[0].Key, t),
			IntervalNumber: intervalNumber,
			IntervalCount:  maxIntervalCount,
		},
		{
			ExposureKey:    decodeKey(source.Keys[1].Key, t),
			IntervalNumber: intervalNumber + maxIntervalCount,
			IntervalCount:  maxIntervalCount,
		},
		{
			ExposureKey:    decodeKey(source.Keys[2].Key, t),
			IntervalNumber: intervalNumber + 2*maxIntervalCount,
			IntervalCount:  maxIntervalCount,
		},
		{
			ExposureKey:    decodeKey(source.Keys[3].Key, t),
			IntervalNumber: intervalNumber + 3*maxIntervalCount,
			IntervalCount:  42,
		},
	}
	batchTime := captureStartTime.Add(time.Hour * 24 * 7)
	batchTimeRounded := TruncateWindow(batchTime)
	for i, v := range want {
		want[i] = &Exposure{
			ExposureKey:      v.ExposureKey,
			TransmissionRisk: 2,
			AppPackageName:   "com.google",
			Regions:          []string{"US", "CA", "MX"},
			IntervalNumber:   v.IntervalNumber,
			IntervalCount:    v.IntervalCount,
			CreatedAt:        batchTimeRounded,
			LocalProvenance:  true,
		}
	}

	allowedAge := 14 * 24 * time.Hour
	transformer, err := NewTransformer(10, allowedAge)
	if err != nil {
		t.Fatalf("NewTransformer returned unexpected error: %v", err)
	}
	got, err := transformer.TransformPublish(source, batchTime)
	if err != nil {
		t.Fatalf("TransformPublish returned unexpected error: %v", err)
	}

	for i := range want {
		if diff := cmp.Diff(want[i], got[i]); diff != "" {
			t.Errorf("TransformPublish mismatch (-want +got):\n%v", diff)
		}
	}
}
