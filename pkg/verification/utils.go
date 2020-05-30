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
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/google/exposure-notifications-server/internal/publish/model"
)

// CalculateExposureKeyHMAC will calculate the verification protocol HMAC value.
// Input keys are already to be base64 encoded. They will be sorted if necessary.
func CalculateExposureKeyHMAC(keys []model.ExposureKey, secret []byte) ([]byte, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("cannot calculate hmac on empty exposure keys")
	}
	// Sort by the key
	sort.Slice(keys, func(i int, j int) bool {
		return strings.Compare(keys[i].Key, keys[j].Key) <= 0
	})

	// Build the cleartext.
	perKeyText := []string{}
	for _, ek := range keys {
		perKeyText = append(perKeyText,
			fmt.Sprintf("%s.%d.%d.%d", ek.Key, ek.IntervalNumber, ek.IntervalCount, ek.TransmissionRisk))
	}

	cleartext := strings.Join(perKeyText, ",")
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(cleartext))

	return mac.Sum(nil), nil
}
