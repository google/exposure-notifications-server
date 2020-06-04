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
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/base64util"
	"github.com/google/exposure-notifications-server/internal/logging"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

// ApplyTransmissionRiskOverrides modifies the transmission risk values in the publish request
// based on the provided TransmissionRiskVector.
// In the live system, the TransmissionRiskVector values come from a trusted public health authority
// and are embedded in the verification certificate (JWT) transmitted on the publish request.
func ApplyTransmissionRiskOverrides(p *verifyapi.Publish, overrides verifyapi.TransmissionRiskVector) {
	if len(overrides) == 0 {
		return
	}
	// The default sort order for TransmissionRiskVector is descending by SinceRollingPeriod.
	sort.Sort(overrides)
	// Sort the keys with the largest start interval first (descending), same as overrides.
	sort.Slice(p.Keys, func(i int, j int) bool {
		return p.Keys[i].IntervalNumber > p.Keys[j].IntervalNumber
	})

	overrideIdx := 0
	for i, eKey := range p.Keys {
		// Advance the overrideIdx until the current key is covered or we exhaust the
		// override index.
		for overrideIdx < len(overrides) &&
			eKey.IntervalNumber+eKey.IntervalCount <= overrides[overrideIdx].SinceRollingPeriod {
			overrideIdx++
		}

		// If we've run out of overrides to apply, then we have to break free.
		if overrideIdx >= len(overrides) {
			break
		}

		// Check to see if this key is in the current override.
		// If the key was EVERY valid during the SinceRollingPeriod then the override applies.
		if eKey.IntervalNumber+eKey.IntervalCount >= overrides[overrideIdx].SinceRollingPeriod {
			p.Keys[i].TransmissionRisk = overrides[overrideIdx].TranismissionRisk
			// don't advance overrideIdx, there might be additional keys in this override.
		}
	}
}

// Exposure represents the record as stored in the database
// TODO(mikehelmick) - refactor this so that there is a public
// Exposure struct that doesn't have public fields and an
// internal struct that does. Separate out the database model
// from direct access.
// Mark records as writable/nowritable - is exposure key encrypted.
type Exposure struct {
	ExposureKey      []byte    `db:"exposure_key"`
	TransmissionRisk int       `db:"transmission_risk"`
	AppPackageName   string    `db:"app_package_name"`
	Regions          []string  `db:"regions"`
	IntervalNumber   int32     `db:"interval_number"`
	IntervalCount    int32     `db:"interval_count"`
	CreatedAt        time.Time `db:"created_at"`
	LocalProvenance  bool      `db:"local_provenance"`
	FederationSyncID int64     `db:"sync_id"`
}

// IntervalNumber calculates the exposure notification system interval
// number based on the input time.
func IntervalNumber(t time.Time) int32 {
	return int32(t.UTC().Unix()) / int32(verifyapi.IntervalLength.Seconds())
}

// TruncateWindow truncates a time based on the size of the creation window.
func TruncateWindow(t time.Time, d time.Duration) time.Time {
	return t.Truncate(d)
}

// TimeForIntervalNumber returns the time at which a specific interval starts.
// This is done by turning the internal number into the corresponding unix timestamp,
// multiplying by 600 seconds (10 minutes).
func TimeForIntervalNumber(interval int32) time.Time {
	return time.Unix(int64(verifyapi.IntervalLength.Seconds())*int64(interval), 0)
}

// Transformer represents a configured Publish -> Exposure[] transformer.
type Transformer struct {
	maxExposureKeys     int
	maxIntervalStartAge time.Duration // How many intervals old does this server accept?
	truncateWindow      time.Duration
	debugRelesaeSameDay bool // If true, still valid keys are not embarged.
}

// NewTransformer creates a transformer for turning publish API requests into
// records for insertion into the database. On the call to TransformPublish
// all data is validated according to the transformer that is used.
func NewTransformer(maxExposureKeys int, maxIntervalStartAge time.Duration, truncateWindow time.Duration, releaseSameDayKeys bool) (*Transformer, error) {
	if maxExposureKeys < 0 || maxExposureKeys > verifyapi.MaxKeysPerPublish {
		return nil, fmt.Errorf("maxExposureKeys must be > 0 and <= %v, got %v", verifyapi.MaxKeysPerPublish, maxExposureKeys)
	}
	return &Transformer{
		maxExposureKeys:     maxExposureKeys,
		maxIntervalStartAge: maxIntervalStartAge,
		truncateWindow:      truncateWindow,
		debugRelesaeSameDay: releaseSameDayKeys,
	}, nil
}

type KeyTransform struct {
	MinStartInterval      int32
	MaxStartInterval      int32
	MaxEndInteral         int32
	CreatedAt             time.Time
	ReleaseStillValidKeys bool
	BatchWindow           time.Duration
}

// TransformExposureKey converts individual key data to an exposure entity.
// Validations during the transform include:
//
// * exposure keys are exactly 16 bytes in length after base64 decoding
// * minInterval <= interval number <= maxInterval
// * MinIntervalCount <= interval count <= MaxIntervalCount
//
func TransformExposureKey(exposureKey verifyapi.ExposureKey, appPackageName string, upcaseRegions []string, settings *KeyTransform) (*Exposure, error) {
	binKey, err := base64util.DecodeString(exposureKey.Key)
	if err != nil {
		return nil, err
	}

	// Validate individual pieces of the exposure key
	if len(binKey) != verifyapi.KeyLength {
		return nil, fmt.Errorf("invalid key length, %v, must be %v", len(binKey), verifyapi.KeyLength)
	}
	if ic := exposureKey.IntervalCount; ic < verifyapi.MinIntervalCount || ic > verifyapi.MaxIntervalCount {
		return nil, fmt.Errorf("invalid interval count, %v, must be >= %v && <= %v", ic, verifyapi.MinIntervalCount, verifyapi.MaxIntervalCount)
	}

	// Validate the IntervalNumber.
	if exposureKey.IntervalNumber < settings.MinStartInterval {
		return nil, fmt.Errorf("interval number %v is too old, must be >= %v", exposureKey.IntervalNumber, settings.MinStartInterval)
	}
	if exposureKey.IntervalNumber > settings.MaxStartInterval {
		return nil, fmt.Errorf("interval number %v is in the future, must be <= %v", exposureKey.IntervalNumber, settings.MaxStartInterval)
	}

	createdAt := settings.CreatedAt
	// If the key is valid beyond the current interval number. Adjust the createdAt time for the key.
	if exposureKey.IntervalNumber+exposureKey.IntervalCount > settings.MaxStartInterval {
		// key is still valid. The created At for this key needs to be adjusted unless debuggin is enabled.
		if !settings.ReleaseStillValidKeys {
			createdAt = TimeForIntervalNumber(exposureKey.IntervalNumber + exposureKey.IntervalCount).Truncate(settings.BatchWindow)
		}
	}

	if tr := exposureKey.TransmissionRisk; tr < verifyapi.MinTransmissionRisk || tr > verifyapi.MaxTransmissionRisk {
		return nil, fmt.Errorf("invalid transmission risk: %v, must be >= %v && <= %v", tr, verifyapi.MinTransmissionRisk, verifyapi.MaxTransmissionRisk)
	}

	return &Exposure{
		ExposureKey:      binKey,
		TransmissionRisk: exposureKey.TransmissionRisk,
		AppPackageName:   appPackageName,
		Regions:          upcaseRegions,
		IntervalNumber:   exposureKey.IntervalNumber,
		IntervalCount:    exposureKey.IntervalCount,
		CreatedAt:        createdAt,
		LocalProvenance:  true,
	}, nil
}

// TransformPublish converts incoming key data to a list of exposure entities.
// The data in the request is validated during the transform, including:
//
// * 0 exposure Keys in the requests
// * > Transformer.maxExposureKeys in the request
//
func (t *Transformer) TransformPublish(ctx context.Context, inData *verifyapi.Publish, batchTime time.Time) ([]*Exposure, error) {
	if t.debugRelesaeSameDay {
		logging.FromContext(ctx).Errorf("DEBUG SERVER - Current day keys are not being embargoed.")
	}

	// Validate the number of keys that want to be published.
	if len(inData.Keys) == 0 {
		return nil, fmt.Errorf("no exposure keys in publish request")
	}
	if len(inData.Keys) > t.maxExposureKeys {
		return nil, fmt.Errorf("too many exposure keys in publish: %v, max of %v is allowed", len(inData.Keys), t.maxExposureKeys)
	}

	defaultCreatedAt := TruncateWindow(batchTime, t.truncateWindow)
	entities := make([]*Exposure, 0, len(inData.Keys))

	settings := KeyTransform{
		// An exposure key must have an interval >= minInterval (max configured age)
		MinStartInterval: IntervalNumber(batchTime.Add(-1 * t.maxIntervalStartAge)),
		// A key must have been issued on the device in the current interval or earlier.
		MaxStartInterval: IntervalNumber(batchTime),
		// And the max valid interval is the maxStartInterval + 144
		MaxEndInteral:         IntervalNumber(batchTime) + verifyapi.MaxIntervalCount,
		CreatedAt:             defaultCreatedAt,
		ReleaseStillValidKeys: t.debugRelesaeSameDay,
		BatchWindow:           t.truncateWindow,
	}

	// Regions are a multi-value property, uppercase them for storage.
	// There is no set of "valid" regions overall, but it is defined
	// elsewhere by what regions an authorized application may write to.
	// See `authorizedapp.Config`
	upcaseRegions := make([]string, len(inData.Regions))
	for i, r := range inData.Regions {
		upcaseRegions[i] = strings.ToUpper(r)
	}

	for _, exposureKey := range inData.Keys {
		exposure, err := TransformExposureKey(exposureKey, inData.AppPackageName, upcaseRegions, &settings)
		if err != nil {
			return nil, fmt.Errorf("invalid publish data: %v", err)
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
