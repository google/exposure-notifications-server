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
	"sort"
	"strings"
)

type Noncer interface {
	// Nonce returns the expected nonce given input data.
	Nonce() string
}

var _ Noncer = (*NonceData)(nil)

type NonceData struct {
	appPackageName string
	ttKeysBase64   []string
	regions        []string
}

func NewNonce(appPackageName string, ttKeysBase64, regions []string) *NonceData {
	// base64 keys are to be lexographically sorted
	sortedKeys := make([]string, len(ttKeysBase64))
	copy(sortedKeys, ttKeysBase64)
	sort.Strings(sortedKeys)

	// regions are to be uppercased and then lexographically sorted
	sortedRegions := make([]string, len(regions))
	for i, r := range regions {
		sortedRegions[i] = strings.ToUpper(r)
	}
	sort.Strings(sortedRegions)

	return &NonceData{
		appPackageName: appPackageName,
		ttKeysBase64:   sortedKeys,
		regions:        sortedRegions,
	}
}

// Nonce returns the expected nonce for this data, from this application.
func (n *NonceData) Nonce() string {
	// The nonce is the appPackageName, keys, and regions put together
	cleartext := n.appPackageName + strings.Join(n.ttKeysBase64, "") + strings.Join(n.regions, "")
	// Take the sha256 checksum of that data
	sum := sha256.Sum256([]byte(cleartext))
	// Base64 encode the result.
	return base64.RawStdEncoding.EncodeToString(sum[:])
}
