// Copyright 2020 the Exposure Notifications Server authors
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
	"github.com/google/exposure-notifications-server/internal/middleware"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/revision"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/verification"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/hashicorp/go-multierror"
)

// Compile-time check to assert this config matches requirements.
var (
	_ setup.AuthorizedAppConfigProvider         = (*Config)(nil)
	_ setup.DatabaseConfigProvider              = (*Config)(nil)
	_ setup.SecretManagerConfigProvider         = (*Config)(nil)
	_ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
	_ model.TransformerConfig                   = (*Config)(nil)
	_ setup.KeyManagerConfigProvider            = (*Config)(nil)
	_ middleware.Maintainable                   = (*Config)(nil)
)

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

	Port        string `env:"PORT, default=8080"`
	Maintenance bool   `env:"MAINTENANCE_MODE, default=false"`

	MaxKeysOnPublish uint `env:"MAX_KEYS_ON_PUBLISH, default=30"`
	// Provides compatibility w/ 1.5 release.
	MaxSameStartIntervalKeys uint          `env:"MAX_SAME_START_INTERVAL_KEYS, default=3"`
	MaxIntervalAge           time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	CreatedAtTruncateWindow  time.Duration `env:"TRUNCATE_WINDOW, default=1h"`

	// Symptom onset settings.
	// Maximum valid range. TEKs presneted with values outside this range, but still "reasonable" will not be saved.
	MaxMagnitudeSymptomOnsetDays uint `env:"MAX_SYMPTOM_ONSET_DAYS, default=14"`
	// MaxValidSymptomOnsetReportDays indicates how many days would be considered
	// a valid symptom onset report (-val..+val). Anything outside
	// that range would be subject to the default symptom onset flags (see below).
	MaxSymptomOnsetReportDays uint `env:"MAX_VALID_SYMPTOM_ONSET_REPORT_DAYS, default=28"`

	// TEKs that arrive without a days since symptom onset (i.e. no symptom onset date),
	// then the upload date minus DEFAULT_SYMPTOM_ONSET_DAYS_AGO is used.
	SymptomOnsetDaysAgo uint `env:"DEFAULT_SYMPTOM_ONSET_DAYS_AGO, default=4"`

	ResponsePaddingMinBytes int64 `env:"RESPONSE_PADDING_MIN_BYTES, default=1024"`
	ResponsePaddingRange    int64 `env:"RESPONSE_PADDING_RANGE, default=1024"`

	RevisionKeyCacheDuration time.Duration `env:"REVISION_KEY_CACHE_DURATION, default=1m"`

	// AllowPartialRevisions permits uploading multiple exposure keys with a
	// revision token where only a subset of the keys are in the token. In that
	// case, only the incoming exposure keys that match the revision token are
	// uploaded and the remainder are discarded.
	AllowPartialRevisions bool `env:"ALLOW_PARTIAL_REVISIONS, default=false"`

	// API Versions.
	EnableV1Alpha1API bool `env:"ENABLE_V1ALPHA1_API, default=false"`

	// If set and if a publish request has no regions (v1alpha1) and the health authority
	// has no regions configured, then this default will be assumed.
	// This is present for an upgrade edgecase where empty region list used to mean "all regions"
	// Should only be set if a server is being operated in a single region.
	DefaultRegion string `env:"DEFAULT_REGION"`

	// LogJSONParseErrors will log errors from parsoning incoming requests if enabled.
	// The logs are at the WARN log level.
	LogJSONParseErrors bool `env:"LOG_JSON_PARSE_ERRORS, default=false"`

	// Flags for local development and testing. This will cause still valid keys
	// to not be embargoed.
	// Normally "still valid" keys can be accepted, but are embargoed.
	ReleaseSameDayKeys      bool `env:"DEBUG_RELEASE_SAME_DAY_KEYS"`
	DebugLogBadCertificates bool `env:"DEBUG_LOG_BAD_CERTIFICATES"`

	// Publish stats API config
	// Minimum number of publish requests that need to be present to see stats for a given day.
	// If the minimum is not met, that day is not revealed or shown in aggregates.
	// This value must be >= 10
	StatsUploadMinimum int64 `env:"STATS_UPLOAD_MINIMUM, default=10"`
	// Allow release of a day's stats after this much time has passed (measured from end of day).
	// Set to <= 0 to disable this feature.
	// If the value is positive, it must be >= 48h.
	StatsEmbargoPeriod           time.Duration `env:"STATS_EMBARGO_PERIOD, default=48h"`
	StatsResponsePaddingMinBytes int64         `env:"RESPONSE_PADDING_MIN_BYTES, default=2048"`
	StatsResponsePaddingRange    int64         `env:"RESPONSE_PADDING_RANGE, default=1024"`

	// ChaffRequestMaxLatencyMS prevents chaff request from consistently increasing latency
	// if the server is under abnormal load.
	ChaffRequestMaxLatencyMS uint64 `env:"CHAFF_REQUEST_MAX_LATENCY_MS, default=1000"`
}

func (c *Config) MaintenanceMode() bool {
	return c.Maintenance
}

func (c *Config) Validate() error {
	var result *multierror.Error

	if c.MaxMagnitudeSymptomOnsetDays == 0 {
		result = multierror.Append(result,
			fmt.Errorf("env var `MAX_SYMPTOM_ONSET_DAYS` must be > 0, got: %v", c.MaxMagnitudeSymptomOnsetDays))
	}
	if c.MaxSymptomOnsetReportDays == 0 {
		result = multierror.Append(result,
			fmt.Errorf("env var `MAX_VALID_SYMPTOM_ONSET_REPORT_DAYS` must be > 0, got: %v", c.MaxSymptomOnsetReportDays))
	}

	if c.StatsUploadMinimum < 10 {
		result = multierror.Append(result,
			fmt.Errorf("env var `STATS_UPLOAD_MINIMUM` must be >= 10, got: %v", c.StatsUploadMinimum))
	}

	if ep := c.StatsEmbargoPeriod; !(ep >= (48*time.Hour) || ep <= 0) {
		result = multierror.Append(result,
			fmt.Errorf("env var `STATS_EMBARGO_PERIOD` must be >= 48h or <= 0 to disable release of stats days below the threshold"))
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
	return c.MaxSymptomOnsetReportDays
}

func (c *Config) DefaultSymptomOnsetDaysAgo() uint {
	return c.SymptomOnsetDaysAgo
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
