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

// Package model is a model abstraction of publish.
package model

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/verification"
	"github.com/google/exposure-notifications-server/pkg/base64util"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

var (
	// ErrorExposureKeyMismatch - internal coding error, tried to revise key A by passing in key B
	ErrorExposureKeyMismatch = fmt.Errorf("attempted to revise a key with a different key")
	// ErrorNonLocalProvenance - key revesion attempted on federated key, which is not allowed
	ErrorNonLocalProvenance = fmt.Errorf("key not origionally uploaded to this server, cannot revise")
	// ErrorKeyAlreadyRevised - attempt to revise a key that has already been revised.
	ErrorKeyAlreadyRevised = fmt.Errorf("key has already been revised and cannot be revised again")
)

// Exposure represents the record as stored in the database
type Exposure struct {
	ExposureKey      []byte
	TransmissionRisk int
	AppPackageName   string
	Regions          []string
	IntervalNumber   int32
	IntervalCount    int32
	CreatedAt        time.Time
	LocalProvenance  bool
	FederationSyncID int64

	// These fileds are nullable to maintain backwards compatibility with
	// older versions that predate their existence.
	HealthAuthorityID     *int64
	ReportType            string
	DaysSinceSymptomOnset *int32

	// Fields to support key revision.
	RevisedReportType            *string
	RevisedAt                    *time.Time
	RevisedDaysSinceSymptomOnset *int32
	RevisedTransmissionRisk      *int

	// b64 key
	base64Key string
}

// Revise updates the Revised fields of a key
func (e *Exposure) Revise(in *Exposure) (bool, error) {
	if e.ExposureKeyBase64() != in.ExposureKeyBase64() {
		return false, ErrorExposureKeyMismatch
	}
	// key doesn't need to be revised if there is no change.
	if e.ReportType == in.ReportType {
		return false, nil
	}
	if !e.LocalProvenance {
		return false, ErrorNonLocalProvenance
	}
	// make sure key hasn't been revised already.
	if e.RevisedAt != nil {
		return false, ErrorKeyAlreadyRevised
	}

	// Check to see if this is a valid transition.
	if !(e.ReportType == verifyapi.ReportTypeClinical && (in.ReportType == verifyapi.ReportTypeConfirmed || in.ReportType == verifyapi.ReportTypeNegative)) {
		return false, fmt.Errorf("invalid report type transition, cannot transition from '%v' to '%v'", e.ReportType, in.ReportType)
	}

	// Update fields.
	// Key is potentially revised by a different health authority.
	e.HealthAuthorityID = in.HealthAuthorityID
	// If there are new regions in the incoming version, add them to the previous on.
	// Regions are not removed however.
	e.AddMissingRegions(in.Regions)
	e.RevisedReportType = &in.ReportType
	e.RevisedAt = &in.CreatedAt
	e.RevisedDaysSinceSymptomOnset = in.DaysSinceSymptomOnset
	e.RevisedTransmissionRisk = &in.TransmissionRisk

	return true, nil
}

func (e *Exposure) AddMissingRegions(regions []string) {
	m := make(map[string]struct{})
	for _, r := range e.Regions {
		m[r] = struct{}{}
	}
	for _, r := range regions {
		if _, ok := m[r]; !ok {
			m[r] = struct{}{}
			e.Regions = append(e.Regions, r)
		}
	}
}

// HasDaysSinceSymptomOnset returns true if the this key has the days since
// symptom onset field is et.
func (e *Exposure) HasDaysSinceSymptomOnset() bool {
	return e.DaysSinceSymptomOnset != nil
}

// SetDaysSinceSymptomOnset sets the days since sympton onset field, possibly
// allocating a new pointer.
func (e *Exposure) SetDaysSinceSymptomOnset(d int32) {
	e.DaysSinceSymptomOnset = &d
}

func (e *Exposure) HasHealthAuthorityID() bool {
	return e.HealthAuthorityID != nil
}

func (e *Exposure) SetHealthAuthorityID(haID int64) {
	e.HealthAuthorityID = &haID
}

func (e *Exposure) HasBeenRevised() bool {
	return e.RevisedAt != nil
}

func (e *Exposure) SetRevisedAt(t time.Time) error {
	if e.RevisedAt != nil {
		return fmt.Errorf("exposure key has already been revised and cannot be revised again")
	}
	e.RevisedAt = &t
	return nil
}

func (e *Exposure) SetRevisedReportType(rt string) {
	e.RevisedReportType = &rt
}

func (e *Exposure) SetRevisedDaysSinceSymptomOnset(d int32) {
	e.RevisedDaysSinceSymptomOnset = &d
}

func (e *Exposure) SetRevisedTransmissionRisk(tr int) {
	e.RevisedTransmissionRisk = &tr
}

// ExposureKeyBase64 returns the ExposuerKey property base64 encoded.
func (e *Exposure) ExposureKeyBase64() string {
	if e.base64Key == "" {
		e.base64Key = base64.StdEncoding.EncodeToString(e.ExposureKey)
	}
	return e.base64Key
}

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
			eKey.IntervalNumber+eKey.IntervalCount <= overrides[overrideIdx].SinceRollingInterval {
			overrideIdx++
		}

		// If we've run out of overrides to apply, then we have to break free.
		if overrideIdx >= len(overrides) {
			break
		}

		// Check to see if this key is in the current override.
		// If the key was EVERY valid during the SinceRollingPeriod then the override applies.
		if eKey.IntervalNumber+eKey.IntervalCount >= overrides[overrideIdx].SinceRollingInterval {
			p.Keys[i].TransmissionRisk = overrides[overrideIdx].TransmissionRisk
			// don't advance overrideIdx, there might be additional keys in this override.
		}
	}
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
// The interval number * 600 (10m = 600s) is the corresponding unix timestamp.
func TimeForIntervalNumber(interval int32) time.Time {
	return time.Unix(int64(verifyapi.IntervalLength.Seconds())*int64(interval), 0)
}

// DaysFromSymptomOnset calculates the number of days between two start intervals.
// Partial days are rounded up/down to the closest day.
// If the checkInterval is before the onsetInterval, number of days will be negative.
func DaysFromSymptomOnset(onsetInterval int32, checkInterval int32) int32 {
	distance := checkInterval - onsetInterval
	days := distance / verifyapi.MaxIntervalCount
	// if the days don't divide evenly, round (up or down) to the closest even day.
	if rem := distance % verifyapi.MaxIntervalCount; rem != 0 {
		// remainder of negative number is negative in go. So if the ABS is more than
		// half a day, adjust the day count.
		if math.Abs(float64(rem)) > verifyapi.MaxIntervalCount/2 {
			// Account for the fact that if day is 0 and rem is > half a day, sign of rem matters.
			if days < 0 || rem < 0 {
				days--
			} else {
				days++
			}
		}
	}
	return days
}

type TransformerConfig interface {
	MaxExposureKeys() uint
	MaxSameDayKeys() uint
	MaxIntervalStartAge() time.Duration
	TruncateWindow() time.Duration
	MaxSymptomOnsetDays() uint
	DebugReleaseSameDayKeys() bool
}

// Transformer represents a configured Publish -> Exposure[] transformer.
type Transformer struct {
	maxExposureKeys     int           // Overall maximum number of keys.
	maxSameDayKeys      int           // Number of keys that are allowed to have the same start interval.
	maxIntervalStartAge time.Duration // How many intervals old does this server accept?
	truncateWindow      time.Duration
	maxSymptomOnsetDays float64 // to avoid casting in comparisons
	debugReleaseSameDay bool    // If true, still valid keys are not embargoed.
}

// NewTransformer creates a transformer for turning publish API requests into
// records for insertion into the database. On the call to TransformPublish
// all data is validated according to the transformer that is used.
func NewTransformer(config TransformerConfig) (*Transformer, error) {
	if config.MaxExposureKeys() <= 0 {
		return nil, fmt.Errorf("maxExposureKeys must be > 0, got %v", config.MaxExposureKeys())
	}
	if config.MaxSameDayKeys() < 1 {
		return nil, fmt.Errorf("maxSameDayKeys must be >= 1, got %v", config.MaxSameDayKeys())
	}
	return &Transformer{
		maxExposureKeys:     int(config.MaxExposureKeys()),
		maxSameDayKeys:      int(config.MaxSameDayKeys()),
		maxIntervalStartAge: config.MaxIntervalStartAge(),
		truncateWindow:      config.TruncateWindow(),
		maxSymptomOnsetDays: float64(config.MaxSymptomOnsetDays()),
		debugReleaseSameDay: config.DebugReleaseSameDayKeys(),
	}, nil
}

// KeyTransform represents the settings to apply when transforming an individual key on a publish request.
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
		// key is still valid. The created At for this key needs to be adjusted unless debugging is enabled.
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

// ReviseKeys takes a set of existing keys, and a list of keys currently being uploaded.
// Only keys that need to be revsised or are being created fir the first time
// are returned in the output set.
func ReviseKeys(ctx context.Context, existing map[string]*Exposure, incoming []*Exposure) ([]*Exposure, error) {
	//logger := logging.FromContext(ctx)
	output := make([]*Exposure, 0, len(incoming))

	// Iterate over incoming keys.
	// If the key already exists
	//  - determine if it needs to be revised, revise it, put in output.
	//  - if it doesn't need to be revised (nochange), don't put in putput
	// New keys, throw it in the output list. Party on.
	for _, inExposure := range incoming {
		prevExposure, ok := existing[inExposure.ExposureKeyBase64()]
		if !ok {
			output = append(output, inExposure)
			continue
		}

		// Attempt to revise this key.
		keyRevised, err := prevExposure.Revise(inExposure)
		if err != nil {
			return nil, err
		}
		if !keyRevised {
			// key hasn't changed, carry on.
			continue
		}
		// Revision worked, add the revised key to the output list.
		output = append(output, prevExposure)
	}

	return output, nil
}

func ReportTypeTransmissionRisk(reportType string, providedTR int) int {
	// If the client provided a transmission risk, we'll use that.
	if providedTR != 0 {
		return providedTR
	}
	// Otherwise this value needs to be backfilled for v1.0 clients.
	switch reportType {
	case verifyapi.ReportTypeConfirmed:
		return verifyapi.TransmissionRiskConfirmedStandard
	case verifyapi.ReportTypeClinical:
		return verifyapi.TransmissionRiskClinical
	case verifyapi.ReportTypeNegative:
		return verifyapi.TransmissionRiskNegative
	}
	return verifyapi.TransmissionRiskUnknown
}

// TransformPublish converts incoming key data to a list of exposure entities.
// The data in the request is validated during the transform, including:
//
// * 0 exposure Keys in the requests
// * > Transformer.maxExposureKeys in the request
//
func (t *Transformer) TransformPublish(ctx context.Context, inData *verifyapi.Publish, claims *verification.VerifiedClaims, batchTime time.Time) ([]*Exposure, error) {
	logger := logging.FromContext(ctx)
	if t.debugReleaseSameDay {
		logger.Errorf("DEBUG SERVER - Current day keys are not being embargoed.")
	}

	// Validate the number of keys that want to be published.
	if len(inData.Keys) == 0 {
		msg := "no exposure keys in publish request"
		logger.Debugf(msg)
		return nil, fmt.Errorf(msg)
	}
	if len(inData.Keys) > t.maxExposureKeys {
		msg := fmt.Sprintf("too many exposure keys in publish: %v, max of %v is allowed", len(inData.Keys), t.maxExposureKeys)
		logger.Debugf(msg)
		return nil, fmt.Errorf(msg)
	}

	if claims != nil {
		ApplyTransmissionRiskOverrides(inData, claims.TransmissionRisks)
	}

	onsetInterval := inData.SymptomOnsetInterval
	if claims != nil && claims.SymptomOnsetInterval > 0 {
		onsetInterval = int32(claims.SymptomOnsetInterval)
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
		ReleaseStillValidKeys: t.debugReleaseSameDay,
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
			logger.Debugf("individual key transform failed: %v", err)
			return nil, fmt.Errorf("invalid publish data: %v", err)
		}
		// If there are verified claims, apply to this key.
		if claims != nil {
			if claims.ReportType != "" {
				exposure.ReportType = claims.ReportType
			}
			exposure.TransmissionRisk = ReportTypeTransmissionRisk(claims.ReportType, exposure.TransmissionRisk)
			if claims.HealthAuthorityID > 0 {
				exposure.SetHealthAuthorityID(claims.HealthAuthorityID)
			}
		}
		// Set days since onset, either from the API or from the verified claims (see above).
		if onsetInterval > 0 {
			daysSince := DaysFromSymptomOnset(onsetInterval, exposure.IntervalNumber)
			if math.Abs(float64(daysSince)) < t.maxSymptomOnsetDays {
				exposure.SetDaysSinceSymptomOnset(daysSince)
			}
		}
		entities = append(entities, exposure)
	}

	// Validate the uploaded data meets configuration parameters.
	// In v1.5+, it is possible to have multiple keys that overlap. They
	// take the form of the same start interval with variable rolling period numbers.
	// Sort by interval number to make necessary checks easier.
	sort.Slice(entities, func(i int, j int) bool {
		if entities[i].IntervalNumber == entities[j].IntervalNumber {
			return entities[i].IntervalCount < entities[j].IntervalCount
		}
		return entities[i].IntervalNumber < entities[j].IntervalNumber
	})
	// Check that any overlapping keys meet configuration.
	// Overlapping keys must have the same start interval. And there is a max number
	// of "same day" keys that are allowed.
	// We do not enforce that keys have UTC midnight aligned start intervals.

	// Running count of start intervals.
	startIntervals := make(map[int32]int)
	lastInterval := entities[0].IntervalNumber
	nextInterval := entities[0].IntervalNumber + entities[0].IntervalCount

	for _, ex := range entities {
		// Relies on the default value of 0 for the map value type.
		startIntervals[ex.IntervalNumber] = startIntervals[ex.IntervalNumber] + 1

		if ex.IntervalNumber == lastInterval {
			// OK, overlaps by start interval. But move out the nextInterval
			nextInterval = ex.IntervalNumber + ex.IntervalCount
			continue
		}

		if ex.IntervalNumber < nextInterval {
			msg := fmt.Sprintf("exposure keys have non aligned overlapping intervals. %v overlaps with previous key that is good from %v to %v.", ex.IntervalNumber, lastInterval, nextInterval)
			logger.Debugf(msg)
			return nil, fmt.Errorf(msg)
		}
		// OK, current key starts at or after the end of the previous one. Advance both variables.
		lastInterval = ex.IntervalNumber
		nextInterval = ex.IntervalNumber + ex.IntervalCount
	}

	for k, v := range startIntervals {
		if v > t.maxSameDayKeys {
			msg := fmt.Sprintf("too many overlapping keys for start interval: %v want: <= %v, got: %v", k, t.maxSameDayKeys, v)
			logger.Debugf(msg)
			return nil, fmt.Errorf(msg)
		}
	}

	return entities, nil
}
