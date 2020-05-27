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
	"errors"
	"strings"
	"time"
)

var (
	ExportBatchOpen     = "OPEN"
	ExportBatchPending  = "PENDING"
	ExportBatchComplete = "COMPLETE"
	ExportBatchDeleted  = "DELETED"
)

const (
	oneDay = 24 * time.Hour
)

type ExportConfig struct {
	ConfigID         int64         `db:"config_id"`
	BucketName       string        `db:"bucket_name"`
	FilenameRoot     string        `db:"filename_root"`
	Period           time.Duration `db:"period_seconds"`
	OutputRegion     string        `db:"output_region"`
	InputRegions     []string      `db:"input_regions"`
	From             time.Time     `db:"from_timestamp"`
	Thru             time.Time     `db:"thru_timestamp"`
	SignatureInfoIDs []int64       `db:"signature_info_ids"`
}

// EffectiveInputRegions either returns `InputRegions` or if that array is
// empty, the output region (`Region`) is returned (in an array).
func (ec *ExportConfig) EffectiveInputRegions() []string {
	return effectiveInputRegions(ec.OutputRegion, ec.InputRegions)
}

func (ec *ExportConfig) InputRegionsOnePerLine() string {
	return strings.Join(ec.InputRegions, "\n")
}

func (ec *ExportConfig) Validate() error {
	if ec.Period > oneDay {
		return errors.New("maximum period is 24h")
	}
	if ec.Period == 0 {
		return errors.New("period must be non-zero")
	}
	if int64(oneDay.Seconds())%int64(ec.Period.Seconds()) != 0 {
		return errors.New("period must divide equally into 24 hours (e.g., 2h, 4h, 12h, 15m, 30m)")
	}
	return nil
}

func (e *ExportConfig) FormattedFromTime() string {
	return e.From.Format(time.UnixDate)
}

func (e *ExportConfig) FormattedThruTime() string {
	if e.Thru.IsZero() {
		return ""
	}
	return e.Thru.Format(time.UnixDate)
}

func (e *ExportConfig) FromHTMLDate() string {
	return toHTMLDate(e.From)
}

func (e *ExportConfig) FromHTMLTime() string {
	return toHTMLTime(e.From)
}

func (e *ExportConfig) ThruHTMLDate() string {
	return toHTMLDate(e.Thru)
}

func (e *ExportConfig) ThruHTMLTime() string {
	return toHTMLTime(e.Thru)
}

type ExportBatch struct {
	BatchID          int64     `db:"batch_id" json:"batchID"`
	ConfigID         int64     `db:"config_id" json:"configID"`
	BucketName       string    `db:"bucket_name" json:"bucketName"`
	FilenameRoot     string    `db:"filename_root" json:"filenameRoot"`
	StartTimestamp   time.Time `db:"start_timestamp" json:"startTimestamp"`
	EndTimestamp     time.Time `db:"end_timestamp" json:"endTimestamp"`
	OutputRegion     string    `db:"region" json:"output_region"`
	InputRegions     []string  `db:"input_regions" json:"inputRegions"`
	Status           string    `db:"status" json:"status"`
	LeaseExpires     time.Time `db:"lease_expires" json:"leaseExpires"`
	SignatureInfoIDs []int64   `db:"signature_info_ids"`
}

// EffectiveInputRegions either returns `InputRegions` or if that array is
// empty, the output region (`Region`) is returned (in an array).
func (eb *ExportBatch) EffectiveInputRegions() []string {
	return effectiveInputRegions(eb.OutputRegion, eb.InputRegions)
}

type ExportFile struct {
	BucketName   string   `db:"bucket_name"`
	Filename     string   `db:"filename"`
	BatchID      int64    `db:"batch_id"`
	OutputRegion string   `db:"output_region"`
	InputRegions []string `db:"input_regions" json:"inputRegions"`
	BatchNum     int      `db:"batch_num"`
	BatchSize    int      `db:"batch_size"`
	Status       string   `db:"status"`
}

// EffectiveInputRegions either returns `InputRegions` or if that array is
// empty, the output region (`Region`) is returned (in an array).
func (ef *ExportFile) EffectiveInputRegions() []string {
	return effectiveInputRegions(ef.OutputRegion, ef.InputRegions)
}

type SignatureInfo struct {
	ID                int64     `db:"id"`
	SigningKey        string    `db:"signing_key"`
	AppPackageName    string    `db:"app_package_name"`
	BundleID          string    `db:"bundle_id"`
	SigningKeyVersion string    `db:"signing_key_version"`
	SigningKeyID      string    `db:"signing_key_id"`
	EndTimestamp      time.Time `db:"thru_timestamp"`
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
	if inRegions != nil && len(inRegions) > 0 {
		return inRegions
	}
	return []string{outRegion}
}
