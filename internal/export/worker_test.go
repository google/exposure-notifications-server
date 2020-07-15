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

package export

import (
	"testing"

	"github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"google.golang.org/protobuf/proto"
)

func TestRandomInt(t *testing.T) {
	expected := make(map[int]bool)
	for i := verifyapi.MinTransmissionRisk; i <= verifyapi.MaxTransmissionRisk; i++ {
		expected[i] = true
	}

	// Run through 1,000 iterations. To ensure the entire range can be hit.
	for i := 0; i < 1000; i++ {
		v, err := randomInt(verifyapi.MinTransmissionRisk, verifyapi.MaxTransmissionRisk)
		if err != nil {
			t.Fatalf("error getting random data")
		}
		if v < verifyapi.MinTransmissionRisk || v > verifyapi.MaxTransmissionRisk {
			t.Fatalf("random data outside expected bounds. %v <= %v <= %v",
				verifyapi.MinTransmissionRisk, v, verifyapi.MaxTransmissionRisk)
		}
		delete(expected, v)
	}
	if len(expected) != 0 {
		t.Fatalf("not all values hit in random range: %v", expected)
	}
}

func TestDoNotPadZeroLength(t *testing.T) {
	exposures := make([]*model.Exposure, 0)
	exposures, err := ensureMinNumExposures(exposures, "US", 1000, 100)
	if err != nil {
		t.Fatalf("unepected error: %v", err)
	}
	if len(exposures) != 0 {
		t.Errorf("empty exposure list got padded, shouldn't have.")
	}
}

func TestEnsureMinExposures(t *testing.T) {
	// Insert a few exposures - that will be used to base the interval information off of.
	exposures := []*model.Exposure{
		{
			ExposureKey:           []byte{1},
			TransmissionRisk:      verifyapi.TransmissionRiskConfirmedStandard,
			IntervalNumber:        123456,
			IntervalCount:         144,
			DaysSinceSymptomOnset: proto.Int32(0),
			ReportType:            verifyapi.ReportTypeConfirmed,
		},
		{
			ExposureKey:           []byte{2},
			TransmissionRisk:      verifyapi.TransmissionRiskConfirmedStandard,
			IntervalNumber:        123456 + 144,
			IntervalCount:         144,
			DaysSinceSymptomOnset: proto.Int32(1),
			ReportType:            verifyapi.ReportTypeConfirmed,
		},
		{
			ExposureKey:           []byte{3},
			TransmissionRisk:      verifyapi.TransmissionRiskConfirmedStandard,
			IntervalNumber:        123456 - 144,
			IntervalCount:         144,
			DaysSinceSymptomOnset: proto.Int32(-1),
			ReportType:            verifyapi.ReportTypeConfirmed,
		},
	}

	numKeys := 100
	variance := 10
	m := make(map[int32]int)
	m[123456-144] = 1
	m[123456] = 1
	m[123456+144] = 1

	// pad the download.
	exposures, err := ensureMinNumExposures(exposures, "US", numKeys, variance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exposures) < numKeys || len(exposures) > numKeys+variance {
		t.Errorf("wrong number of exposures, want: >=%v and <=%v, got: %v", numKeys, numKeys+variance, len(exposures))
	}

	for _, e := range exposures {
		m[e.IntervalNumber] = m[e.IntervalNumber] + 1
	}
	for k, v := range m {
		if v < 20 {
			t.Errorf("distribution not random, expected >= 30 keys with start interval %v, got %v", k, v)
		}
	}
}
