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

package model

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/base64util"
)

const (
	// 21 Days worth of keys is the maximum per publish request (inclusive)
	maxKeysPerPublish = 21

	// only valid exposure key keyLength
	KeyLength = 16

	// Transmission risk constraints (inclusive..inclusive)
	MinTransmissionRisk = 0 // 0 indicates, no/unknown risk.
	MaxTransmissionRisk = 8

	// Intervals are defined as 10 minute periods, there are 144 of them in a day.
	// IntervalCount constraints (inclusive..inclusive)
	MinIntervalCount = 1
	MaxIntervalCount = 144

	// Self explanitory.
	oneDay = time.Hour * 24

	// interval length
	intervalLength = 10 * time.Minute
)

// Publish represents the body of the PublishInfectedIds API call.
// Keys: Required and must have length >= 1 and <= 21 (`maxKeysPerPublish`)
// Regions: Array of regions. System defined, must match configuration.
// AppPackageName: The identifier for the mobile application.
//  - Android: The App Package AppPackageName
//  - iOS: The BundleID
// TransmissionRisk: An integer from 0-8 (inclusive) that represnets
//  the transmission risk for this publish.
// Verification: The attestation payload for this request. (iOS or Android specific)
//   Base64 encoded.
// VerificationAuthorityName: a string that should be verified against the code provider.
//  Note: This project doesn't directly include a diagnosis code verification System
//        but does provide the ability to configure one in `serverevn.ServerEnv`
type Publish struct {
	Keys                      []ExposureKey `json:"temporaryExposureKeys"`
	Regions                   []string      `json:"regions"`
	AppPackageName            string        `json:"appPackageName"`
	Platform                  string        `json:"platform"`
	DeviceVerificationPayload string        `json:"deviceVerificationPayload"`
	VerificationPayload       string        `json:"verificationPayload"`
	Padding                   string        `json:"padding"`
}

// AndroidNonce returns the Android. This ensures that the data in the request
// is the same data that was used to create the device attestation.
func (p *Publish) AndroidNonce() string {
	// base64 keys are to be lexographically sorted
	sortedKeys := make([]ExposureKey, len(p.Keys))
	copy(sortedKeys, p.Keys)
	sort.Slice(sortedKeys, func(i int, j int) bool {
		return sortedKeys[i].Key < sortedKeys[j].Key
	})

	// regions are to be uppercased and then lexographically sorted
	sortedRegions := make([]string, len(p.Regions))
	for i, r := range p.Regions {
		sortedRegions[i] = strings.ToUpper(r)
	}
	sort.Strings(sortedRegions)

	keys := make([]string, 0, len(sortedKeys))
	for _, k := range sortedKeys {
		keys = append(keys, fmt.Sprintf("%v.%v.%v.%v", k.Key, k.IntervalNumber, k.IntervalCount, k.TransmissionRisk))
	}

	// The cleartext is a combination of all of the data on the request
	// in a specific order.
	//
	// appPackageName|key[,key]|region[,region]|verificationAuthorityName
	// Keys are ancoded as
	//     base64(exposureKey).itnervalNumber.IntervalCount.transmissionRisk
	// When there is > 1 key, keys are comma separated.
	// Keys must in sorted order based on the sorting of the base64 exposure key.
	// Regions are uppercased, sorted, and comma sepreated
	cleartext :=
		p.AppPackageName + "|" +
			strings.Join(keys, ",") + "|" + // where key is b64key.intervalNum.intervalCount
			strings.Join(sortedRegions, ",") + "|" +
			p.VerificationPayload

	// Take the sha256 checksum of that data
	sum := sha256.Sum256([]byte(cleartext))

	// Base64 encode the result.
	return base64.StdEncoding.EncodeToString(sum[:])
}

// ExposureKey is the 16 byte key, the start time of the key and the
// duration of the key. A duration of 0 means 24 hours.
// - ALL fields are REQUIRED and must meet the constraints below.
// Key must be the base64 (RFC 4648) encoded 16 byte exposure key from the device.
// - Base64 encoding should include padding, as per RFC 4648
// - if the key is not exactly 16 bytes in length, the request will be failed
// - that is, the whole batch will fail.
// IntervalNumber must be "reasonable" as in the system won't accept keys that
//   are scheduled to start in the future or that are too far in the past, which
//   is configurable per installation.
// IntervalCount must >= `minIntervalCount` and <= `maxIntervalCount`
//   1 - 144 inclusive.
// transmissionRisk must be >= 0 and <= 8
type ExposureKey struct {
	Key              string `json:"key"`
	IntervalNumber   int32  `json:"rollingStartNumber"`
	IntervalCount    int32  `json:"rollingPeriod"`
	TransmissionRisk int    `json:"transmissionRisk"`
}

// Exposure represents the record as storedin the database
// TODO(mikehelmick) - refactor this so that there is a public
// Exposure struct that doesn't have public fields and an
// internal struct that does. Separate out the database model
// from direct access.
// Mark records as writable/nowritable - is exposure key encrypted
type Exposure struct {
	ExposureKey               []byte    `db:"exposure_key"`
	TransmissionRisk          int       `db:"transmission_risk"`
	AppPackageName            string    `db:"app_package_name"`
	Regions                   []string  `db:"regions"`
	IntervalNumber            int32     `db:"interval_number"`
	IntervalCount             int32     `db:"interval_count"`
	CreatedAt                 time.Time `db:"created_at"`
	LocalProvenance           bool      `db:"local_provenance"`
	VerificationAuthorityName string    `db:"verification_authority_name"`
	FederationSyncID          int64     `db:"sync_id"`
}

// IntervalNumber calculates the exposure notification system interval
// number based on the input time.
func IntervalNumber(t time.Time) int32 {
	return int32(t.UTC().Unix()) / int32(intervalLength.Seconds())
}

// TruncateWindow truncates a time based on the size of the creation window.
func TruncateWindow(t time.Time, d time.Duration) time.Time {
	return t.Truncate(d)
}

// Transformer represents a configured Publish -> Exposure[] transformer.
type Transformer struct {
	maxExposureKeys     int
	maxIntervalStartAge time.Duration // How many intervals old does this server accept?
	truncateWindow      time.Duration
}

// NewTransformer creates a transformer for turning publish API requests into
// records for insertion into the database. On the call to TransofmrPublish
// all data is validated according to the transformer that is used.
func NewTransformer(maxExposureKeys int, maxIntervalStartAge time.Duration, truncateWindow time.Duration) (*Transformer, error) {
	if maxExposureKeys < 0 || maxExposureKeys > maxKeysPerPublish {
		return nil, fmt.Errorf("maxExposureKeys must be > 0 and <= %v, got %v", maxKeysPerPublish, maxExposureKeys)
	}
	return &Transformer{
		maxExposureKeys:     maxExposureKeys,
		maxIntervalStartAge: maxIntervalStartAge,
		truncateWindow:      truncateWindow,
	}, nil
}

// TransformPublish converts incoming key data to a list of exposure entities.
// The data in the request is validated during the transform, including:
//
// * 0 exposure Keys in the requests
// * > Transormer.maxExposureKeys in the request
// * exposure keys that aren't exactly 16 bytes in length after base64 decoding
//
func (t *Transformer) TransformPublish(inData *Publish, batchTime time.Time) ([]*Exposure, error) {
	// Validate the number of keys that want to be published.
	if len(inData.Keys) == 0 {
		return nil, fmt.Errorf("no exposure keys in publish request")
	}
	if len(inData.Keys) > t.maxExposureKeys {
		return nil, fmt.Errorf("too many exposure keys in publish: %v, max of %v is allowed", len(inData.Keys), t.maxExposureKeys)
	}

	createdAt := TruncateWindow(batchTime, t.truncateWindow)
	entities := make([]*Exposure, 0, len(inData.Keys))

	// An exposure key must have an interval >= minInteravl (max configured age)
	minIntervalNumber := IntervalNumber(batchTime.Add(-1 * t.maxIntervalStartAge))
	// And have an interval <= maxInterval (configured allowed clock skew)
	maxIntervalNumber := IntervalNumber(batchTime)

	// Regions are a multi-value property, uppercase them for storage.
	// There is no set of "valid" regions overall, but it is defined
	// elsewhere by what regions an authorized application may write to.
	// See `apiconfig.APIConfig`
	upcaseRegions := make([]string, len(inData.Regions))
	for i, r := range inData.Regions {
		upcaseRegions[i] = strings.ToUpper(r)
	}

	for _, exposureKey := range inData.Keys {
		binKey, err := base64util.DecodeString(exposureKey.Key)
		if err != nil {
			return nil, err
		}

		// Validate individual pieces of this publish request.
		if len(binKey) != KeyLength {
			return nil, fmt.Errorf("invalid key length, %v, must be %v", len(binKey), KeyLength)
		}
		if ic := exposureKey.IntervalCount; ic < MinIntervalCount || ic > MaxIntervalCount {
			return nil, fmt.Errorf("invalid interval count, %v, must be >= %v && <= %v", ic, MinIntervalCount, MaxIntervalCount)
		}

		// Validate the IntervalNumber.
		if exposureKey.IntervalNumber < minIntervalNumber {
			return nil, fmt.Errorf("interval number %v is too old, must be >= %v", exposureKey.IntervalNumber, minIntervalNumber)
		}
		if exposureKey.IntervalNumber >= maxIntervalNumber {
			return nil, fmt.Errorf("interval number %v is in the future, must be < %v", exposureKey.IntervalNumber, maxIntervalNumber)
		}

		if tr := exposureKey.TransmissionRisk; tr < MinTransmissionRisk || tr > MaxTransmissionRisk {
			return nil, fmt.Errorf("invalid transmission risk: %v, must be >= %v && <= %v", tr, MinTransmissionRisk, MaxTransmissionRisk)
		}

		exposure := &Exposure{
			ExposureKey:      binKey,
			TransmissionRisk: exposureKey.TransmissionRisk,
			AppPackageName:   inData.AppPackageName,
			Regions:          upcaseRegions,
			IntervalNumber:   exposureKey.IntervalNumber,
			IntervalCount:    exposureKey.IntervalCount,
			CreatedAt:        createdAt,
			LocalProvenance:  true,
		}
		entities = append(entities, exposure)
	}

	// Ensure that the uploaded keys are for a consecutive time period. No
	// overlaps and no gaps.
	// 1) Sort by interval number.
	sort.Slice(entities, func(i int, j int) bool {
		return entities[i].IntervalNumber < entities[j].IntervalNumber
	})
	// 2) Walk the slice and verify no gaps/overlaps.
	// We know the slice isn't empty, seed w/ the first interval.
	nextInterval := entities[0].IntervalNumber
	for _, ex := range entities {
		if ex.IntervalNumber < nextInterval {
			return nil, fmt.Errorf("exposure keys have overlapping intervals")
		}
		nextInterval = ex.IntervalNumber + ex.IntervalCount
	}

	return entities, nil
}
