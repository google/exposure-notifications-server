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

// Package publish defines the exposure keys publishing API.
package publish

import (
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/verification"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/hashicorp/go-multierror"
)

// Compile-time check to assert this config matches requirements.
var _ setup.AuthorizedAppConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)
var _ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
var _ model.TransformerConfig = (*Config)(nil)
var _ setup.KeyManagerConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the publish components.
type Config struct {
	AuthorizedApp         authorizedapp.Config
	Database              database.Config
	SecretManager         secrets.Config
	KeyManager            keys.Config
	Verification          verification.Config
	ObservabilityExporter observability.Config
	RevisionToken         revision.Config

	Port             string `env:"PORT, default=8080"`
	MaxKeysOnPublish uint   `env:"MAX_KEYS_ON_PUBLISH, default=30"`
	// Provides compatibility w/ 1.5 release.
	MaxSameStartIntervalKeys uint          `env:"MAX_SAME_START_INTERVAL_KEYS, default=3"`
	MaxIntervalAge           time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	CreatedAtTruncateWindow  time.Duration `env:"TRUNCATE_WINDOW, default=1h"`

	// Symptom onset settings.
	// Maximum valid range. TEKs presneted with values outside this range, but still "reasonable" will not be saved.
	MaxMagnitudeSymptomOnsetDays uint `env:"MAX_SYMPTOM_ONSET_DAYS, default=14"`
	// MaxValidSypmtomOnsetReportDays indicates how many days would be considered
	// a valid symptom onset report (-val..+val). Anything outside
	// that range would be subject to the default symptom onset flags (see below).
	MaxSypmtomOnsetReportDays uint `env:"MAX_VALID_SYMPOTOM_ONSET_REPORT_DAYS, default=28"`

	// If set, TEKs that arrive without a days since symptom onset (i.e. no symptom onset date)
	// will be set to the default symptom onset days.
	UseDefaultSymptomOnset bool `env:"USE_DEFAULT_SYMPTOM_ONSET_DAYS, default=true"`
	SymptomOnsetDays       uint `env:"DEFAULT_SYMPTOM_ONSET_DAYS, default=10"`

	ResponsePaddingMinBytes int64 `env:"RESPONSE_PADDING_MIN_BYTES, default=1024"`
	ResponsePaddingRange    int64 `env:"RESPONSE_PADDING_RANGE, default=1024"`

	RevisionKeyCacheDuration time.Duration `env:"REVISION_KEY_CACHE_DURATION, default=1m"`

	// AllowPartialRevisions permits uploading multiple exposure keys with a
	// revision token where only a subset of the keys are in the token. In that
	// case, only the incoming exposure keys that match the revision token are
	// uploaded and the remainder are discarded.
	AllowPartialRevisions bool `env:"ALLOW_PARTIAL_REVISIONS, default=false"`

	// API Versions.
	EnableV1Alpha1API bool `env:"ENABLE_V1ALPHA1_API, default=true"`

	// If set and if a publish request has no regions (v1alpha1) and the health authority
	// has no regions configured, then this default will be assumed.
	// This is present for an upgrade edgecase where empty region list used to mean "all regions"
	// Should only be set if a server is being operated in a single region.
	DefaultRegion string `env:"DEFAULT_REGION"`

	// Feature flags - eventually these are removed as features become default behavior
	FailOnCertificateAudienceMismatch bool `env:"FEATURE_FAIL_ON_CERTIFICATE_AUDIENCE_MISMATCH, default=true"`

	// Flags for local development and testing. This will cause still valid keys
	// to not be embargoed.
	// Normally "still valid" keys can be accepted, but are embargoed.
	ReleaseSameDayKeys      bool `env:"DEBUG_RELEASE_SAME_DAY_KEYS"`
	DebugLogBadCertificates bool `env:"DEBUG_LOG_BAD_CERTIFICATES"`
}

func (c *Config) Validate() error {
	var result *multierror.Error

	if c.MaxMagnitudeSymptomOnsetDays == 0 {
		result = multierror.Append(result,
			fmt.Errorf("env var `MAX_SYMPTOM_ONSET_DAYS` must be > 0, got: %v", c.MaxMagnitudeSymptomOnsetDays))
	}
	if c.MaxSypmtomOnsetReportDays == 0 {
		result = multierror.Append(result,
			fmt.Errorf("env var `MAX_VALID_SYMPOTOM_ONSET_REPORT_DAYS` must be > 0, got: %v", c.MaxSypmtomOnsetReportDays))
	}

	return result.ErrorOrNil()
}

func (c *Config) MaxExposureKeys() uint {
	return c.MaxKeysOnPublish
}

func (c *Config) MaxSameDayKeys() uint {
	return c.MaxSameStartIntervalKeys
}

func (c *Config) MaxIntervalStartAge() time.Duration {
	return c.MaxIntervalAge
}

func (c *Config) TruncateWindow() time.Duration {
	return c.CreatedAtTruncateWindow
}

func (c *Config) MaxSymptomOnsetDays() uint {
	return c.MaxMagnitudeSymptomOnsetDays
}

func (c *Config) MaxValidSymptomOnsetReportDays() uint {
	return c.MaxSypmtomOnsetReportDays
}

func (c *Config) UseDefaultSymptomOnsetDays() bool {
	return c.UseDefaultSymptomOnset
}

func (c *Config) DefaultSymptomOnsetDays() int32 {
	return int32(c.SymptomOnsetDays)
}

func (c *Config) DebugReleaseSameDayKeys() bool {
	return c.ReleaseSameDayKeys
}

func (c *Config) AuthorizedAppConfig() *authorizedapp.Config {
	return &c.AuthorizedApp
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}

func (c *Config) KeyManagerConfig() *keys.Config {
	return &c.KeyManager
}
