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

// Package model is a model abstraction of exports.
package model

import (
	"errors"
	"strings"
	"time"
)

var (
	ExportBatchOpen          = "OPEN"
	ExportBatchPending       = "PENDING"
	ExportBatchComplete      = "COMPLETE"
	ExportBatchDeletePending = "DEL_PEND"
	ExportBatchDeleted       = "DELETED"
)

const (
	oneDay = 24 * time.Hour
)

// ExportConfig describes what goes into an export, and how frequently.
// These are used to periodically generate an ExportBatch.
type ExportConfig struct {
	ConfigID           int64
	BucketName         string
	FilenameRoot       string
	Period             time.Duration
	OutputRegion       string
	InputRegions       []string
	ExcludeRegions     []string
	IncludeTravelers   bool
	OnlyNonTravelers   bool
	From               time.Time
	Thru               time.Time
	SignatureInfoIDs   []int64
	MaxRecordsOverride *int
}

// EffectiveInputRegions either returns `InputRegions` or if that array is
// empty, the output region (`Region`) is returned (in an array).
func (ec *ExportConfig) EffectiveInputRegions() []string {
	return effectiveInputRegions(ec.OutputRegion, ec.InputRegions)
}

func (ec *ExportConfig) InputRegionsOnePerLine() string {
	return strings.Join(ec.InputRegions, "\n")
}

func (ec *ExportConfig) ExcludeRegionsOnePerLine() string {
	return strings.Join(ec.ExcludeRegions, "\n")
}

func (ec *ExportConfig) Validate() error {
	if ec.Period > oneDay {
		return errors.New("maximum period is 24h")
	}
	if ec.Period < 1*time.Second {
		return errors.New("period must be at least 1 second")
	}
	if int64(oneDay.Seconds())%int64(ec.Period.Seconds()) != 0 {
		return errors.New("period must divide equally into 24 hours (e.g., 2h, 4h, 12h, 15m, 30m)")
	}
	return nil
}

// ExportBatch holds what was used to generate an export.
type ExportBatch struct {
	BatchID            int64
	ConfigID           int64
	BucketName         string
	FilenameRoot       string
	StartTimestamp     time.Time
	EndTimestamp       time.Time
	OutputRegion       string
	InputRegions       []string
	IncludeTravelers   bool
	OnlyNonTravelers   bool
	ExcludeRegions     []string
	Status             string
	LeaseExpires       time.Time
	SignatureInfoIDs   []int64
	MaxRecordsOverride *int
}

// EffectiveMaxRecords returns either the provided value or the override
// present in this config.
func (eb *ExportBatch) EffectiveMaxRecords(systemDefault int) int {
	// If the value is null in the database or > 0. Prevent accidental setting to 0 to
	// think it indicates default.
	if eb.MaxRecordsOverride != nil && *eb.MaxRecordsOverride > 0 {
		return *eb.MaxRecordsOverride
	}
	return systemDefault
}

// EffectiveInputRegions either returns `InputRegions` or if that array is
// empty, the output region (`Region`) is returned (in an array).
func (eb *ExportBatch) EffectiveInputRegions() []string {
	return effectiveInputRegions(eb.OutputRegion, eb.InputRegions)
}

type ExportFile struct {
	BucketName       string
	Filename         string
	BatchID          int64
	OutputRegion     string
	InputRegions     []string
	IncludeTravelers bool
	OnlyNonTravelers bool
	ExcludeRegions   []string
	BatchNum         int
	BatchSize        int
	Status           string
}

// EffectiveInputRegions either returns `InputRegions` or if that array is
// empty, the output region (`Region`) is returned (in an array).
func (ef *ExportFile) EffectiveInputRegions() []string {
	return effectiveInputRegions(ef.OutputRegion, ef.InputRegions)
}

type SignatureInfo struct {
	ID                int64
	SigningKey        string
	SigningKeyVersion string
	SigningKeyID      string
	EndTimestamp      time.Time
}

// FormattedEndTimestamp returns the end date for display in the admin console.
func (s *SignatureInfo) FormattedEndTimestamp() string {
	if s.EndTimestamp.IsZero() {
		return ""
	}
	return s.EndTimestamp.UTC().Format(time.UnixDate)
}

// HTMLEndDate returns EndDate in a format for the HTML date input default value.
func (s *SignatureInfo) HTMLEndDate() string {
	return toHTMLDate(s.EndTimestamp)
}

// HTMLEndTime returns EndDate in a format for the HTML time input default value.
func (s *SignatureInfo) HTMLEndTime() string {
	return toHTMLTime(s.EndTimestamp)
}

func toHTMLDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func toHTMLTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("15:04")
}

func effectiveInputRegions(outRegion string, inRegions []string) []string {
	if len(inRegions) > 0 {
		return inRegions
	}
	return []string{outRegion}
}
