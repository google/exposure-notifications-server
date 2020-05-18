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
	"crypto/rand"
	"fmt"
	"testing"
)

func TestRandomInt(t *testing.T) {
	expected := make(map[int]bool)
	for i := model.MinTransmissionRisk; i <= model.MaxTransmissionRisk; i++ {
		expected[i] = true
	}

	// Run through 1,000 iterations. To ensure the entire range can be hit.
	for i := 0; i < 1000; i++ {
		v, err := randomInt(model.MinTransmissionRisk, model.MaxTransmissionRisk)
		if err != nil {
			t.Fatalf("error getting random data")
		}
		if v < model.MinTransmissionRisk || v > model.MaxTransmissionRisk {
			t.Fatalf("random data outside expected bounds. %v <= %v <= %v",
				model.MinTransmissionRisk, v, model.MaxTransmissionRisk)
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

func addExposure(t *testing.T, exposures []*model.Exposure, interval, count int32, risk int) []*model.Exposure {
	key := make([]byte, model.KeyLength)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("error getting random data: %v", err)
	}
	return append(exposures,
		&model.Exposure{
			ExposureKey:      key,
			TransmissionRisk: risk,
			IntervalNumber:   interval,
			IntervalCount:    count,
		})
}

func TestEnsureMinExposures(t *testing.T) {
	expectedTR := make(map[int]bool)
	for i := model.MinTransmissionRisk; i <= model.MaxTransmissionRisk; i++ {
		expectedTR[i] = true
	}
	// Insert a few exposures - that will be used to base the interval information off of.
	exposures := make([]*model.Exposure, 0)
	exposures = addExposure(t, exposures, 123456, 144, 0)
	exposures = addExposure(t, exposures, 789101, 88, 0)
	// all of these combinations are expected
	eIntervals := make(map[string]bool)
	eIntervals["123456.144"] = true // covered by input
	eIntervals["123456.88"] = false
	eIntervals["789101.144"] = false
	eIntervals["789101.88"] = true

	// pad the download.
	exposures, err := ensureMinNumExposures(exposures, "US", 2000, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exposures) < 2000 || len(exposures) > 2010 {
		t.Errorf("wrong number of exposures, want: >=2000 and <=2010, got: %v", len(exposures))
	}

	for _, e := range exposures {
		delete(expectedTR, e.TransmissionRisk)
		eIntervals[fmt.Sprintf("%v.%v", e.IntervalNumber, e.IntervalCount)] = true
	}
	if len(expectedTR) != 0 {
		t.Errorf("Didn't cover all expected transmission risks in batch of 1000: %v", expectedTR)
	}
	if got, want := len(eIntervals), 4; got != want {
		t.Errorf("Unexpected number of intervalNum/count combinations, got %d, want %d", got, want)
	}
	for k, v := range eIntervals {
		if !v {
			t.Errorf("interval %v was not found in output", k)
		}
	}
}
