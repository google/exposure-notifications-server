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

// This package is a CLI tool for generating test exposure key data.
package util

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"math/big"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
)

const (
	// the length of a diagnosis key, always 16 bytes
	dkLen               = 16
	maxTransmissionRisk = 8
)

func RandomIntervalCount() int32 {
	n, err := rand.Int(rand.Reader, big.NewInt(144))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	return int32(n.Int64() + 1) // valid values are 1-144
}

func RandomInt(maxValue int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxValue)))
	if err != nil {
		log.Fatalf("rand.Int: %v", err)
	}
	return int(n.Int64())
}

func RandomTransmissionRisk() int {
	return RandomInt(maxTransmissionRisk) + 1
}

func RandomArrValue(arr []string) string {
	return arr[RandomInt(len(arr))]
}

func GenerateExposureKeys(numKeys, tr int) []database.ExposureKey {
	// When publishing multiple keys - they'll be on different days.
	intervalCount := RandomIntervalCount()
	intervalNumber := int32(time.Now().Unix()/600) - intervalCount
	exposureKeys := make([]database.ExposureKey, numKeys)
	for i, rawKey := range keys {
		transmissionRisk := tr
		if transmissionRisk < 0 {
			transmissionRisk = RandomTransmissionRisk()
		}
		exposureKeys[i] = RandomExposureKey(intervalNumber, intervalCount, transmissionRisk)

		// Adjust interval math for next key.
		intervalCount = RandomIntervalCount()
		intervalNumber -= intervalCount
	}
	return exposureKeys
}

// Creates a random exposure key.
func RandomExposureKey(intervalNumber Interval, intervalCount int32, transmissionRisk int) database.ExposureKey {
        return ExposureKey(GenerateKey(), intervalNumber, intervalCount, transmissionRisk)
}

// Generates the random byte sequence.
func RandomBytes(arrLen int) []byte {
        padding := make([]byte, arrLen)
        _, err := rand.Read(padding)
        if err != nil {
                log.Fatalf("error generating padding: %v", err)
        }
        return padding
}

func GenerateKey() string {
        return ToBase64(RandomBytes(dkLen))
}

// Encodes bytes array to base64.
func ToBase64(key []byte) string {
        return base64.StdEncoding.EncodeToString(key)
}

// Decodes base64 string to []byte.
func DecodeKey(b64key string) []byte {
        k, err := base64.StdEncoding.DecodeString(b64key)
        if err != nil {
                log.Fatalf("unable to decode key: %v", err)
        }
        return k
}
