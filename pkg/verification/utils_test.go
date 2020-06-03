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

package verification

import (
	"encoding/base64"
	"testing"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

func TestCalculateHac(t *testing.T) {
	secret := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	eKeys := []verifyapi.ExposureKey{
		{
			Key:              "z2Cx9hdz2SlxZ8GEgqTYpA==",
			IntervalNumber:   1,
			IntervalCount:    144,
			TransmissionRisk: 3,
		},
		{
			Key:              "dPCphLzfG4uzXneNimkPRQ==",
			IntervalNumber:   144,
			IntervalCount:    144,
			TransmissionRisk: 5,
		},
	}

	mac, err := CalculateExposureKeyHMAC(eKeys, secret)
	if err != nil {
		t.Fatal(err)
	}

	got := base64.StdEncoding.EncodeToString(mac)
	want := "2u1nHt5WWurJytFLF3xitNzM99oNrad2y4YGOL53AeY="
	// Normally, to verify we would calculate this again, and verify with
	// hmac.Equals. This is just verifying the calculation code in this package.
	if got != want {
		t.Fatalf("incorrect mac, want: %v, got %v", want, mac)
	}
}
