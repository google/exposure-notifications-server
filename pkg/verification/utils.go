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

// Package verification provides verification utilities.
//
// This is provided as reference to application authors wishing to calculate
// the exposure key HMAC as part of their exposure notifications mobile app.
//
// This protocol is detailed at
// https://developers.google.com/android/exposure-notifications/verification-system
//
// Although exported, this package is non intended for general consumption.
// It is a shared dependency between multiple exposure notifications projects.
// We cannot guarantee that there won't be breaking changes in the future.
package verification

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

// CalculateExposureKeyHMAC will calculate the verification protocol HMAC value.
// Input keys are already to be base64 encoded. They will be sorted if necessary.
func CalculateExposureKeyHMAC(keys []verifyapi.ExposureKey, secret []byte) ([]byte, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("cannot calculate hmac on empty exposure keys")
	}
	// Sort by the key
	sort.Slice(keys, func(i int, j int) bool {
		return strings.Compare(keys[i].Key, keys[j].Key) <= 0
	})

	// Build the cleartext.
	perKeyText := make([]string, 0, len(keys))
	for _, ek := range keys {
		perKeyText = append(perKeyText,
			fmt.Sprintf("%s.%d.%d.%d", ek.Key, ek.IntervalNumber, ek.IntervalCount, ek.TransmissionRisk))
	}

	cleartext := strings.Join(perKeyText, ",")
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(cleartext)); err != nil {
		return nil, fmt.Errorf("failed to write hmac: %w", err)
	}

	return mac.Sum(nil), nil
}
