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

package android

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/google/exposure-notifications-server/internal/model"
)

// Noncer definces an interface for providing Nonce strings.
type Noncer interface {
	// Nonce returns the expected nonce given input data.
	Nonce() string
}

// Compile-time check to assert NonceData implements the Noncer interface.
var _ Noncer = (*nonceData)(nil)

type nonceData struct {
	appPackageName   string
	transmissionRisk int
	ttKeysBase64     []model.ExposureKey
	regions          []string
	verification     string
}

// NewNonce Creates a new `Noncer{}` based on the inbound publish request.
// This ensures that the data in the request is the same data that was used
// to create the device attestation.
func NewNonce(publish *model.Publish) Noncer {
	// base64 keys are to be lexographically sorted
	sortedKeys := make([]model.ExposureKey, len(publish.Keys))
	copy(sortedKeys, publish.Keys)
	sort.Slice(sortedKeys, func(i int, j int) bool {
		return sortedKeys[i].Key < sortedKeys[j].Key
	})

	// regions are to be uppercased and then lexographically sorted
	sortedRegions := make([]string, len(publish.Regions))
	for i, r := range publish.Regions {
		sortedRegions[i] = strings.ToUpper(r)
	}
	sort.Strings(sortedRegions)

	return &nonceData{
		appPackageName:   publish.AppPackageName,
		transmissionRisk: publish.TransmissionRisk,
		ttKeysBase64:     sortedKeys,
		regions:          sortedRegions,
		verification:     publish.VerificationPayload,
	}
}

// Nonce returns the expected nonce for this data, from this application.
func (n *nonceData) Nonce() string {
	keys := make([]string, 0, len(n.ttKeysBase64))
	for _, k := range n.ttKeysBase64 {
		keys = append(keys, fmt.Sprintf("%v.%v.%v", k.Key, k.IntervalNumber, k.IntervalCount))
	}

	// The cleartext is a combination of all of the data on the request
	// in a specific order.
	//
	// appPackageName|transmissionRisk|key[,key]|region[,region]|verificationAuthorityName
	// Keys are ancoded as
	//     base64(exposureKey).itnervalNumber.IntervalCount
	// When there is > 1 key, keys are comma separated.
	// Keys must in sorted order based on the sorting of the base64 exposure key.
	// Regions are uppercased, sorted, and comma sepreated
	cleartext :=
		n.appPackageName + "|" +
			fmt.Sprintf("%v", n.transmissionRisk) + "|" +
			strings.Join(keys, ",") + "|" + // where key is b64key.intervalNum.intervalCount
			strings.Join(n.regions, ",") + "|" +
			n.verification

	// Take the sha256 checksum of that data
	sum := sha256.Sum256([]byte(cleartext))
	// Base64 encode the result.
	return base64.StdEncoding.EncodeToString(sum[:])
}
