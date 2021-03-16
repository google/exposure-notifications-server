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

	"github.com/google/exposure-notifications-server/internal/pb/export"
	"github.com/google/exposure-notifications-server/internal/verification"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/hashicorp/go-multierror"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
)

var (
	// ErrorExposureKeyMismatch - internal coding error, tried to revise key A by passing in key B
	ErrorExposureKeyMismatch = fmt.Errorf("attempted to revise a key with a different key")
	// ErrorNonLocalProvenance - key revision attempted on federated key, which is not allowed
	ErrorNonLocalProvenance = fmt.Errorf("key not origionally uploaded to this server, cannot revise")
	// ErrorNotSameFederationSource - if a key arrived by federation, it can only be be revised by the same query (same source)
	ErrorNotSameFederationSource = fmt.Errorf("key cannot be revised by a different federation query")
	// ErrorKeyAlreadyRevised - attempt to revise a key that has already been revised.
	ErrorKeyAlreadyRevised = fmt.Errorf("key has already been revised and cannot be revised again")
)

var _ error = (*ErrorKeyInvalidReportTypeTransition)(nil)

// ErrorKeyInvalidReportTypeTransition is an error returned when the TEK tried
// to move to an invalid state (e.g. positive -> likely).
type ErrorKeyInvalidReportTypeTransition struct {
	from, to string
}

// Error implements error.
func (e *ErrorKeyInvalidReportTypeTransition) Error() string {
	return fmt.Sprintf("invalid report type transition: cannot transition from %q to %q",
		e.from, e.to)
}

// Exposure represents the record as stored in the database
type Exposure struct {
	ExposureKey       []byte
	TransmissionRisk  int
	AppPackageName    string
	Regions           []string
	Traveler          bool
	IntervalNumber    int32
	IntervalCount     int32
	CreatedAt         time.Time
	LocalProvenance   bool
	FederationSyncID  int64
	FederationQueryID string
	// Export file based Federation
	ExportImportID      *int64
	ImportFileID        *int64
	RevisedImportFileID *int64

	// These fields are nullable to maintain backwards compatibility with
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

// ExportImportConfig represents the configuration for
// processing of export files from other systems and how they are imported
// into this server.
type ExportImportConfig struct {
	DefaultReportType         string
	BackfillSymptomOnset      bool
	BackfillSymptomOnsetValue int32
	MaxSymptomOnsetDays       int32
	AllowClinical             bool
	AllowRevoked              bool
}

// FromExportKey is used to read a key from an export file and convert it back to the
// internal database format.
func FromExportKey(key *export.TemporaryExposureKey, config *ExportImportConfig) (*Exposure, error) {
	exp := &Exposure{
		ExposureKey: make([]byte, verifyapi.KeyLength),
	}
	if len(key.KeyData) != verifyapi.KeyLength {
		return nil, fmt.Errorf("invalid key length")
	}
	copy(exp.ExposureKey, key.KeyData)

	if key.ReportType != nil {
		rt := *key.ReportType
		switch rt {
		case export.TemporaryExposureKey_CONFIRMED_TEST:
			exp.ReportType = verifyapi.ReportTypeConfirmed
		case export.TemporaryExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS:
			exp.ReportType = verifyapi.ReportTypeClinical
		case export.TemporaryExposureKey_REVOKED:
			exp.ReportType = verifyapi.ReportTypeNegative
		case export.TemporaryExposureKey_UNKNOWN:
			exp.ReportType = ""
		default:
			return nil, fmt.Errorf("unsupported report type: %s",
				export.TemporaryExposureKey_ReportType_name[int32(rt)])
		}
	}
	if exp.ReportType == "" && config.DefaultReportType != "" {
		exp.ReportType = config.DefaultReportType
	}

	if !config.AllowRevoked && exp.ReportType == verifyapi.ReportTypeNegative {
		return nil, fmt.Errorf("saw revoked key when not allowed")
	}
	if !config.AllowClinical && exp.ReportType == verifyapi.ReportTypeClinical {
		return nil, fmt.Errorf("saw likely key when not allowed")
	}

	//nolint:staticcheck // SA1019: may be set on v1 files.
	if key.TransmissionRiskLevel != nil {
		if tr := *key.TransmissionRiskLevel; tr < verifyapi.MinTransmissionRisk {
			return nil, fmt.Errorf("transmission risk too low: %d, must be >= %d", tr, verifyapi.MinTransmissionRisk)
		} else if tr > verifyapi.MaxTransmissionRisk {
			return nil, fmt.Errorf("transmission risk too high: %d, must be <= %d", tr, verifyapi.MaxTransmissionRisk)
		} else {
			exp.TransmissionRisk = int(tr)
		}
	}
	// Apply the TR backfill defaults based on report type.
	exp.TransmissionRisk = ReportTypeTransmissionRisk(exp.ReportType, exp.TransmissionRisk)

	if key.RollingStartIntervalNumber == nil {
		return nil, fmt.Errorf("missing rolling_start_interval_number")
	}
	exp.IntervalNumber = *key.RollingStartIntervalNumber

	if key.RollingPeriod == nil {
		exp.IntervalCount = verifyapi.MaxIntervalCount
	} else {
		if rp := *key.RollingPeriod; rp < verifyapi.MinIntervalCount {
			return nil, fmt.Errorf("rolling period too low: %d must be >= %d", rp, verifyapi.MinIntervalCount)
		} else if rp > verifyapi.MaxIntervalCount {
			return nil, fmt.Errorf("rolling period too high: %d must be <= %d", rp, verifyapi.MaxIntervalCount)
		} else {
			exp.IntervalCount = rp
		}
	}

	if key.DaysSinceOnsetOfSymptoms != nil {
		dsos := *key.DaysSinceOnsetOfSymptoms
		if dsos >= (-1*config.MaxSymptomOnsetDays) && dsos <= config.MaxSymptomOnsetDays {
			exp.DaysSinceSymptomOnset = &dsos
		} else {
			return nil, fmt.Errorf("days since onset of symptoms is out of range: %d must be within: %d", dsos, config.MaxSymptomOnsetDays)
		}
	}
	// make sure that something is set for days since symptom onset.
	// since the upload date and original test/symptom dates are not known, apply
	// a configurable default value (if enabled)
	if exp.DaysSinceSymptomOnset == nil && config.BackfillSymptomOnset {
		exp.DaysSinceSymptomOnset = &config.BackfillSymptomOnsetValue
	}

	exp.LocalProvenance = false

	return exp, nil
}

// ValidReportTypeTransition checks if a TEK is allowed to transition
// from the `from` type to the `to` type.
func ValidReportTypeTransition(from, to string) bool {
	if from == verifyapi.ReportTypeClinical {
		return to == verifyapi.ReportTypeConfirmed || to == verifyapi.ReportTypeNegative
	}
	if from == verifyapi.ReportTypeSelfReport {
		return to == verifyapi.ReportTypeConfirmed || to == verifyapi.ReportTypeClinical || to == verifyapi.ReportTypeNegative
	}
	return false
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
	// Key is being published again, but has already been revised to target report type.
	if e.RevisedAt != nil && e.RevisedReportType != nil && *e.RevisedReportType == in.ReportType {
		return false, nil
	}
	if !e.LocalProvenance {
		nonLocalOK := false
		if e.ExportImportID != nil {
			if in.ExportImportID == nil || *e.ExportImportID != *in.ExportImportID {
				return false, ErrorNotSameFederationSource
			}
			nonLocalOK = true
		} else if e.FederationQueryID != "" {
			if e.FederationQueryID != in.FederationQueryID {
				return false, ErrorNotSameFederationSource
			}
			nonLocalOK = true
		}

		if !nonLocalOK {
			return false, ErrorNonLocalProvenance
		}
	}
	// make sure key hasn't been revised already.
	if e.RevisedAt != nil {
		return false, ErrorKeyAlreadyRevised
	}

	// Check to see if this is a valid transition.
	eReportType := e.ReportType
	if eReportType == "" {
		eReportType = verifyapi.ReportTypeClinical
	}
	if !ValidReportTypeTransition(eReportType, in.ReportType) {
		return false, &ErrorKeyInvalidReportTypeTransition{
			from: e.ReportType,
			to:   in.ReportType,
		}
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
	tr := ReportTypeTransmissionRisk(in.ReportType, in.TransmissionRisk)
	e.RevisedTransmissionRisk = &tr

	e.HealthAuthorityID = in.HealthAuthorityID
	e.RevisedImportFileID = in.ImportFileID

	return true, nil
}

// AddMissingRegions will merge the input regions into the regions already on the exposure.
// Set union operation.
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

// SetDaysSinceSymptomOnset sets the days since symptom onset field, possibly
// allocating a new pointer.
func (e *Exposure) SetDaysSinceSymptomOnset(d int32) {
	e.DaysSinceSymptomOnset = &d
}

// HasHealthAuthorityID returns true if this Exposure has a health authority ID.
func (e *Exposure) HasHealthAuthorityID() bool {
	return e.HealthAuthorityID != nil
}

// SetHealthAuthorityID assigned a health authority ID. Typically done during transform.
func (e *Exposure) SetHealthAuthorityID(haID int64) {
	e.HealthAuthorityID = &haID
}

// HasBeenRevised returns true if this key has been revised. This is indicated
// by the RevisedAt time not being nil.
func (e *Exposure) HasBeenRevised() bool {
	return e.RevisedAt != nil
}

// SetRevisedAt will set the revision time on this Exposure. The RevisedAt timestamp
// can only be set once. Attempting to set it again will result in an error.
func (e *Exposure) SetRevisedAt(t time.Time) error {
	if e.RevisedAt != nil {
		return fmt.Errorf("exposure key has already been revised and cannot be revised again")
	}
	e.RevisedAt = &t
	return nil
}

// SetRevisedReportType will set the revised report type.
func (e *Exposure) SetRevisedReportType(rt string) {
	e.RevisedReportType = &rt
}

// SetRevisedDaysSinceSymptomOnset will set the revised days since symptom onset.
func (e *Exposure) SetRevisedDaysSinceSymptomOnset(d int32) {
	e.RevisedDaysSinceSymptomOnset = &d
}

// SetRevisedTransmissionRisk will set the revised transmission risk.
func (e *Exposure) SetRevisedTransmissionRisk(tr int) {
	e.RevisedTransmissionRisk = &tr
}

// ExposureKeyBase64 returns the ExposureKey property base64 encoded.
func (e *Exposure) ExposureKeyBase64() string {
	if e.base64Key == "" {
		e.base64Key = base64.StdEncoding.EncodeToString(e.ExposureKey)
	}
	return e.base64Key
}

// AdjustAndValidate both validates the kay and if necessary makes adjustments
// to the timing field (createdAt).
func (e *Exposure) AdjustAndValidate(settings *KeyTransform) error {
	// Validate individual pieces of the exposure key
	if l := len(e.ExposureKey); l != verifyapi.KeyLength {
		return fmt.Errorf("invalid key length, %v, must be %v", l, verifyapi.KeyLength)
	}
	if ic := e.IntervalCount; ic < verifyapi.MinIntervalCount || ic > verifyapi.MaxIntervalCount {
		return fmt.Errorf("invalid interval count, %v, must be >= %v && <= %v", ic, verifyapi.MinIntervalCount, verifyapi.MaxIntervalCount)
	}

	// Validate the IntervalNumber, if the key was ever valid during this period, we'll accept it.
	if validUntil := e.IntervalNumber + e.IntervalCount; validUntil < settings.MinStartInterval {
		return fmt.Errorf("key expires before minimum window; %v + %v = %v which is too old, must be >= %v", e.IntervalNumber, e.IntervalCount, validUntil, settings.MinStartInterval)
	}
	if e.IntervalNumber > settings.MaxStartInterval {
		return fmt.Errorf("interval number %v is in the future, must be <= %v", e.IntervalNumber, settings.MaxStartInterval)
	}

	// If the key is valid beyond the current interval number. Adjust the createdAt time for the key.
	if e.IntervalNumber+e.IntervalCount > settings.MaxStartInterval {
		// key is still valid. The created At for this key needs to be adjusted unless debugging is enabled.
		if !settings.ReleaseStillValidKeys {
			// The add of the batch window is to ensure that the created at time is after the expiry.
			e.CreatedAt = TimeForIntervalNumber(e.IntervalNumber + e.IntervalCount).Add(settings.BatchWindow).Truncate(settings.BatchWindow)
		}
	}

	if tr := e.TransmissionRisk; tr < verifyapi.MinTransmissionRisk || tr > verifyapi.MaxTransmissionRisk {
		return fmt.Errorf("invalid transmission risk: %v, must be >= %v && <= %v", tr, verifyapi.MinTransmissionRisk, verifyapi.MaxTransmissionRisk)
	}

	return nil
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

// DaysBetweenIntervals calculates the number of days between two start intervals.
// The intervals represent their start interval (UTC midnight). Partial days always
// round up.
func DaysBetweenIntervals(a int32, b int32) int32 {
	a = IntervalNumber(timeutils.UTCMidnight(TimeForIntervalNumber(a)))
	b = IntervalNumber(timeutils.UTCMidnight(TimeForIntervalNumber(b)))
	distance := b - a
	// This will always divide evenly since a and b are aligned on UTC midnight boundaries.
	days := distance / verifyapi.MaxIntervalCount
	return days
}

// TransformerConfig defines the interface that is needed to configure a `Transformer`
type TransformerConfig interface {
	MaxExposureKeys() uint
	MaxSameDayKeys() uint
	MaxIntervalStartAge() time.Duration
	TruncateWindow() time.Duration
	MaxSymptomOnsetDays() uint
	MaxValidSymptomOnsetReportDays() uint
	DefaultSymptomOnsetDaysAgo() uint
	DebugReleaseSameDayKeys() bool
}

// Transformer represents a configured Publish -> Exposure[] transformer.
type Transformer struct {
	maxExposureKeys                int           // Overall maximum number of keys.
	maxSameDayKeys                 int           // Number of keys that are allowed to have the same start interval.
	maxIntervalStartAge            time.Duration // How many intervals old does this server accept?
	truncateWindow                 time.Duration
	maxSymptomOnsetDays            float64 // to avoid casting in comparisons
	maxValidSymptomOnsetReportDays uint
	defaultSymptomOnsetDaysAgo     uint
	debugReleaseSameDay            bool // If true, still valid keys are not embargoed.
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
		maxExposureKeys:                int(config.MaxExposureKeys()),
		maxSameDayKeys:                 int(config.MaxSameDayKeys()),
		maxIntervalStartAge:            config.MaxIntervalStartAge(),
		truncateWindow:                 config.TruncateWindow(),
		maxSymptomOnsetDays:            float64(config.MaxSymptomOnsetDays()),
		maxValidSymptomOnsetReportDays: config.MaxValidSymptomOnsetReportDays(),
		defaultSymptomOnsetDaysAgo:     config.DefaultSymptomOnsetDaysAgo(),
		debugReleaseSameDay:            config.DebugReleaseSameDayKeys(),
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
// * minInterval <= interval number +intervalCount <= maxInterval
// * MinIntervalCount <= interval count <= MaxIntervalCount
func TransformExposureKey(exposureKey verifyapi.ExposureKey, appPackageName string, uppercaseRegions []string, settings *KeyTransform) (*Exposure, error) {
	binKey, err := base64util.DecodeString(exposureKey.Key)
	if err != nil {
		return nil, err
	}

	e := &Exposure{
		ExposureKey:      binKey,
		TransmissionRisk: exposureKey.TransmissionRisk,
		AppPackageName:   appPackageName,
		Regions:          uppercaseRegions,
		IntervalNumber:   exposureKey.IntervalNumber,
		IntervalCount:    exposureKey.IntervalCount,
		CreatedAt:        settings.CreatedAt,
		LocalProvenance:  true,
	}

	if err := e.AdjustAndValidate(settings); err != nil {
		return nil, err
	}
	return e, nil
}

// ReviseKeys takes a set of existing keys, and a list of keys currently being uploaded.
// Only keys that need to be revised or are being created for the first time
// are returned in the output set.
func ReviseKeys(ctx context.Context, existing map[string]*Exposure, incoming []*Exposure) ([]*Exposure, error) {
	output := make([]*Exposure, 0, len(incoming))

	// Iterate over incoming keys.
	// If the key already exists
	//  - determine if it needs to be revised, revise it, put in output.
	//  - if it doesn't need to be revised (nochange), don't put in output
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

// ReportTypeTransmissionRisk will calculate the backfill, default Transmission Risk.
// If there is a provided transmission risk that is non-zero, that will be used, otherwise
// this mapping is used:
// * Confirmed Test -> 2
// * Clinical Diagnosis -> 4
// * Negative -> 6
// See constants defined in
// pkg/api/v1alpha1/verification_types.go
func ReportTypeTransmissionRisk(reportType string, providedTR int) int {
	// If the client provided a transmission risk, we'll use that.
	if providedTR != 0 {
		return providedTR
	}
	// Otherwise this value needs to be backfilled for verifyapi.0 clients.
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

type TransformPublishResult struct {
	Exposures   []*Exposure
	PublishInfo *PublishInfo
	Warnings    []string
}

// TransformPublish converts incoming key data to a list of exposure entities.
// The data in the request is validated during the transform, including:
//
// * 0 exposure Keys in the requests
// * > Transformer.maxExposureKeys in the request
//
// The return params are the list of exposures, a list of warnings, and any
// errors that occur.
//
func (t *Transformer) TransformPublish(ctx context.Context, inData *verifyapi.Publish, regions []string, claims *verification.VerifiedClaims, batchTime time.Time) (*TransformPublishResult, error) {
	logger := logging.FromContext(ctx).Named("TransformPublish")

	if t.debugReleaseSameDay {
		logger.Warnw("DEBUG SERVER - CURRENT DAYS KEYS ARE NOT EMBARGOED!")
	}

	// Validate the number of keys that want to be published.
	if len(inData.Keys) == 0 {
		msg := "no exposure keys in publish request"
		logger.Debugf(msg)
		return &TransformPublishResult{}, fmt.Errorf(msg)
	}
	if len(inData.Keys) > t.maxExposureKeys {
		msg := fmt.Sprintf("too many exposure keys in publish: %v, max of %v is allowed", len(inData.Keys), t.maxExposureKeys)
		logger.Debugf(msg)
		return &TransformPublishResult{}, fmt.Errorf(msg)
	}

	defaultCreatedAt := TruncateWindow(batchTime, t.truncateWindow)
	entities := make([]*Exposure, 0, len(inData.Keys))

	// Some of the stats of the publish request can be calculated in line with the transform.
	// Some won't matter until after the save, so this structure is created
	// here and returned for further updating.
	stats := &PublishInfo{
		CreatedAt: defaultCreatedAt,
	}

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

	// For validating key timing information, can't be newer than now.
	currentInterval := IntervalNumber(batchTime)
	// For validating the passed in symptom interval, relative to current time.
	minSymptomInterval := IntervalNumber(
		timeutils.UTCMidnight(timeutils.SubtractDays(batchTime, t.maxValidSymptomOnsetReportDays)))

	// Base level, assume there is no symptom onset interval present.
	onsetInterval := int32(0)
	if pubInt := inData.SymptomOnsetInterval; pubInt < currentInterval && pubInt >= minSymptomInterval {
		onsetInterval = pubInt
	} else if claims != nil {
		if vcInt := int32(claims.SymptomOnsetInterval); vcInt < currentInterval && vcInt >= minSymptomInterval {
			// If the symtom onset interval provided on publish is too old to be relevant
			// and one was provided in the verification certificate, take that one.
			onsetInterval = int32(claims.SymptomOnsetInterval)
		}
	}
	// If we reach this point, and onsetInterval is 0 OR if the onset interval
	// is "unreasonable" then we default the onsetInterval to 4 (*configurable)
	// days ago to approximate symptom onset.
	//
	// There are launched applications using this sever that rely on this
	// behavior - that are passing invalid symptom onset interviews, those
	// are screened about above when the onsetInterval is set.
	if daysSince := math.Abs(float64(DaysBetweenIntervals(onsetInterval, currentInterval))); onsetInterval == 0 || daysSince > float64(t.maxValidSymptomOnsetReportDays) {
		logger.Debugw("defaulting days since symptom onset")
		onsetInterval = IntervalNumber(timeutils.SubtractDays(batchTime, t.defaultSymptomOnsetDaysAgo))
		stats.MissingOnset = true
	}

	// If an onset was provided, that should be put in the stats for this publish.
	if !stats.MissingOnset {
		stats.OnsetDaysAgo = int(DaysBetweenIntervals(onsetInterval, currentInterval))
	}

	// Regions are a multi-value property, uppercase them for storage.
	// There is no set of "valid" regions overall, but it is defined
	// elsewhere by what regions an authorized application may write to.
	// See `authorizedapp.Config`
	uppercaseRegions := make([]string, len(regions))
	for i, r := range regions {
		uppercaseRegions[i] = strings.ToUpper(r)
	}

	var transformWarnings []string
	var transformErrors *multierror.Error
	for i, exposureKey := range inData.Keys {
		exposure, err := TransformExposureKey(exposureKey, inData.HealthAuthorityID, uppercaseRegions, &settings)
		if err != nil {
			logger.Debugw("individual key transform failed", "error", err)
			transformErrors = multierror.Append(transformErrors, fmt.Errorf("key %d cannot be imported: %w", i, err))
			continue
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
			daysSince := DaysBetweenIntervals(onsetInterval, exposure.IntervalNumber)
			// Note that previously this returned an error, but this broke the iOS
			// implementation since it is unable to handle partial success. As such,
			// it was converted to a warning that's a separate field in the API
			// response.
			if abs := math.Abs(float64(daysSince)); abs > t.maxSymptomOnsetDays {
				logger.Debugw("setting days since symptom onset to null on key due to symptom onset magnitude too high", "daysSince", daysSince)
				transformWarnings = append(transformWarnings, fmt.Sprintf("key %d symptom onset is too large, %v > %v - saving without this key", i, abs, t.maxSymptomOnsetDays))
				continue
			}

			// The value is within acceptable range, save it.
			exposure.SetDaysSinceSymptomOnset(daysSince)
		}

		// Check and see many days old the key is.
		if daysOld := DaysBetweenIntervals(exposure.IntervalNumber, currentInterval); daysOld > int32(stats.OldestDays) {
			stats.OldestDays = int(daysOld)
		}

		exposure.Traveler = inData.Traveler
		entities = append(entities, exposure)
	}

	if len(entities) == 0 {
		// All keys in the batch are invalid.
		return &TransformPublishResult{
			Warnings: transformWarnings,
		}, transformErrors.ErrorOrNil()
	}

	// Validate the uploaded data meets configuration parameters.
	// In verifyapi.5+, it is possible to have multiple keys that overlap. They
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
			return &TransformPublishResult{
				Warnings: transformWarnings,
			}, fmt.Errorf(msg)
		}
		// OK, current key starts at or after the end of the previous one. Advance both variables.
		lastInterval = ex.IntervalNumber
		nextInterval = ex.IntervalNumber + ex.IntervalCount
	}

	for k, v := range startIntervals {
		if v > t.maxSameDayKeys {
			msg := fmt.Sprintf("too many overlapping keys for start interval: %v want: <= %v, got: %v", k, t.maxSameDayKeys, v)
			logger.Debugf(msg)
			return &TransformPublishResult{
				Warnings: transformWarnings,
			}, fmt.Errorf(msg)
		}
	}

	return &TransformPublishResult{
		Exposures:   entities,
		PublishInfo: stats,
		Warnings:    transformWarnings,
	}, transformErrors.ErrorOrNil()
}
