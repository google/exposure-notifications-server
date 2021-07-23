// Copyright 2020 the Exposure Notifications Server authors
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

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

// CalculateExpsureKeyHMACv1Alpha1 is a convenience method for anyone still on v1alpha1.
// Deprecated: use CalculateExposureKeyHMAC instead
// Preserved for clients on v1alpha1, will be removed in v0.3 release.
func CalculateExpsureKeyHMACv1Alpha1(legacyKeys []v1alpha1.ExposureKey, secret []byte) ([]byte, error) {
	keys := make([]verifyapi.ExposureKey, len(legacyKeys))
	for i, k := range legacyKeys {
		keys[i] = verifyapi.ExposureKey{
			Key:              k.Key,
			IntervalNumber:   k.IntervalNumber,
			IntervalCount:    k.IntervalCount,
			TransmissionRisk: k.TransmissionRisk,
		}
	}
	return CalculateExposureKeyHMAC(keys, secret)
}

// CalculateAllAllowedExposureKeyHMAC calculates the main HMAC and the optional HMAC. The optional HMAC
// is only valid if the transmission risks are all zero.
func CalculateAllAllowedExposureKeyHMAC(keys []verifyapi.ExposureKey, secret []byte) ([][]byte, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("cannot calculate hmac on empty exposure keys")
	}
	// Sort by the key
	sort.Slice(keys, func(i int, j int) bool {
		return strings.Compare(keys[i].Key, keys[j].Key) <= 0
	})

	// Build the cleartext.
	perKeyText := make([]string, 0, len(keys))
	altPerKeyText := make([]string, 0, len(keys))
	calculateAlt := true
	for _, ek := range keys {
		perKeyText = append(perKeyText,
			fmt.Sprintf("%s.%d.%d.%d", ek.Key, ek.IntervalNumber, ek.IntervalCount, ek.TransmissionRisk))
		altPerKeyText = append(altPerKeyText,
			fmt.Sprintf("%s.%d.%d", ek.Key, ek.IntervalNumber, ek.IntervalCount))
		// The alt HMAC is only valid of all transmission risk are "omitted" (set to zero).
		calculateAlt = calculateAlt && ek.TransmissionRisk == 0
	}

	cleartext := strings.Join(perKeyText, ",")
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(cleartext)); err != nil {
		return nil, fmt.Errorf("failed to write hmac: %w", err)
	}
	results := [][]byte{mac.Sum(nil)}

	if calculateAlt {
		altCleartext := strings.Join(altPerKeyText, ",")
		mac := hmac.New(sha256.New, secret)
		if _, err := mac.Write([]byte(altCleartext)); err != nil {
			return nil, fmt.Errorf("failed to write hmac: %w", err)
		}
		results = append(results, mac.Sum(nil))
	}

	return results, nil
}

// CalculateExposureKeyHMAC will calculate the verification protocol HMAC value.
// Input keys are already to be base64 encoded. They will be sorted if necessary.
func CalculateExposureKeyHMAC(keys []verifyapi.ExposureKey, secret []byte) ([]byte, error) {
	results, err := CalculateAllAllowedExposureKeyHMAC(keys, secret)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}
