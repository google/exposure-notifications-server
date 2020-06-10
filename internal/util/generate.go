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

// Package util is a CLI tool for generating test exposure key data.
package util

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"math/big"
	"time"

	"github.com/google/exposure-notifications-server/internal/base64util"
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/testing/enclient"
)

const (
	// the length of a diagnosis key, always 16 bytes
	dkLen            = 16
	maxIntervalCount = 144
)

// RandomIntervalCount produces a random interval.
func RandomIntervalCount() (int32, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(maxIntervalCount))
	if err != nil {
		return 0, err
	}
	return int32(n.Int64() + 1), nil // valid values start at 1
}

// RandomInt produces a random integer up to but not including maxValue.
func RandomInt(maxValue int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxValue)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// RandomIntWithMin is inclusive, [min:max].
func RandomIntWithMin(min, max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + min, nil
}

// RandomTransmissionRisk produces a random transmission risk score.
func RandomTransmissionRisk() (int, error) {
	n, err := RandomInt(v1alpha1.MaxTransmissionRisk)
	return n + 1, err
}

// RandomArrValue chooses a random element from the array.
func RandomArrValue(arr []string) (string, error) {
	n, err := RandomInt(len(arr))
	if err != nil {
		return "", err
	}
	return arr[n], nil
}

// GenerateExposureKeys creates the given number of exposure keys.
func GenerateExposureKeys(numKeys, tr int, randomInterval bool) []v1alpha1.ExposureKey {
	// When publishing multiple keys - they'll be on different days.
	var err error
	intervalCount := int32(144)
	if randomInterval {
		intervalCount, err = RandomIntervalCount()
		if err != nil {
			log.Fatalf("problem with random interval: %v", err)
		}
	}
	// Keys will normally align to UTC day boundries.
	utcDay := time.Now().UTC().Truncate(24 * time.Hour)
	intervalNumber := int32(utcDay.Unix()/600) - intervalCount
	exposureKeys := make([]v1alpha1.ExposureKey, numKeys)
	for i := 0; i < numKeys; i++ {
		transmissionRisk := tr
		if transmissionRisk < 0 {
			transmissionRisk, err = RandomTransmissionRisk()
			if err != nil {
				log.Fatalf("problem with transmission risk: %v", err)
			}
		}
		exposureKeys[i], err = RandomExposureKey(enclient.Interval(intervalNumber), intervalCount, transmissionRisk)
		if err != nil {
			log.Fatalf("problem creating random exposure key: %v", err)
		}

		// Adjust interval math for next key.
		if randomInterval {
			intervalCount, err = RandomIntervalCount()
			if err != nil {
				log.Fatalf("problem with random interval: %v", err)
			}
		}
		intervalNumber -= intervalCount
	}
	return exposureKeys
}

// RandomExposureKey creates a random exposure key.
func RandomExposureKey(intervalNumber enclient.Interval, intervalCount int32, transmissionRisk int) (v1alpha1.ExposureKey, error) {
	key, err := GenerateKey()
	if err != nil {
		return v1alpha1.ExposureKey{}, err
	}
	return v1alpha1.ExposureKey{
		Key:              key,
		IntervalNumber:   int32(intervalNumber),
		IntervalCount:    intervalCount,
		TransmissionRisk: transmissionRisk,
	}, nil
}

// RandomBytes generates a random byte sequence.
func RandomBytes(arrLen int) ([]byte, error) {
	padding := make([]byte, arrLen)
	_, err := rand.Read(padding)
	if err != nil {
		return nil, err
	}
	return padding, nil
}

// GenerateKey generates a key.
func GenerateKey() (string, error) {
	b, err := RandomBytes(dkLen)
	if err != nil {
		return "", err
	}
	return ToBase64(b), nil
}

// ToBase64 encodes bytes array to base64.
func ToBase64(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodeKey decodes base64 string to []byte.
func DecodeKey(b64key string) []byte {
	k, err := base64util.DecodeString(b64key)
	if err != nil {
		log.Fatalf("unable to decode key: %v", err)
	}
	return k
}
