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
	"encoding/base64"
	"strings"
	"time"
)

const (
	// Intervals are defined as 10 minute periods, there are 144 of them in a day.
	maxIntervalCount = 144
)

// Publish represents the body of the PublishInfectedIds API call.
type Publish struct {
	Keys             []ExposureKey `json:"exposureKeys"`
	Regions          []string      `json:"regions"`
	AppPackageName   string        `json:"appPackageName"`
	TransmissionRisk int           `json:"transmissionRisk"`
	Verification     string        `json:"verificationPayload"`
	// TODO(helmick): validate this field
	VerificationAuthorityName string `json:"verificationAuthorityName"`
}

// ExposureKey is the 16 byte key, the start time of the key and the
// duration of the key. A duration of 0 means 24 hours.
type ExposureKey struct {
	Key            string `json:"key"`
	IntervalNumber int32  `json:"intervalNumber"`
	IntervalCount  int32  `json:"intervalCount"`
}

// Infection represents the record as storedin the database
// TODO(helmick) - refactor this so that there is a public
// Infection struct that doesn't have public fields and an
// internal struct that does. Separate out the database model
// from direct access.
// Mark records as writable/nowritable - is exposure key encrypted
type Infection struct {
	ExposureKey               []byte    `db:"exposure_key"`
	TransmissionRisk          int       `db:"transmission_risk"`
	AppPackageName            string    `db:"app_package_name"`
	Regions                   []string  `db:"regions"`
	IntervalNumber            int32     `db:"interval_number"`
	IntervalCount             int32     `db:"interval_count"`
	CreatedAt                 time.Time `db:"created_at"`
	LocalProvenance           bool      `db:"local_provenance"`
	VerificationAuthorityName string    `db:"verification_authority_name"`
	FederationSyncID          string    `db:"sync_id"`
}

const (
	oneDay       = time.Hour * 24
	createWindow = time.Minute * 15
)

// TruncateWindow truncates a time based on the size of the creation window.
func TruncateWindow(t time.Time) time.Time {
	return t.Truncate(createWindow)
}

func correctIntervalCount(count int32) int32 {
	if count <= 0 || count > maxIntervalCount {
		return maxIntervalCount
	}
	return count
}

// TransformPublish converts incoming key data to a list of infection entities.
func TransformPublish(inData *Publish, batchTime time.Time) ([]*Infection, error) {
	createdAt := TruncateWindow(batchTime)
	entities := make([]*Infection, 0, len(inData.Keys))

	// Regions are a multi-value property, uppercase them for storage.
	upcaseRegions := make([]string, len(inData.Regions))
	for i, r := range inData.Regions {
		upcaseRegions[i] = strings.ToUpper(r)
	}

	for _, exposureKey := range inData.Keys {
		binKey, err := base64.StdEncoding.DecodeString(exposureKey.Key)
		if err != nil {
			return nil, err
		}
		// TODO(helmick) - data validation
		infection := &Infection{
			ExposureKey:               binKey,
			TransmissionRisk:          inData.TransmissionRisk,
			AppPackageName:            inData.AppPackageName,
			Regions:                   upcaseRegions,
			IntervalNumber:            exposureKey.IntervalNumber,
			IntervalCount:             correctIntervalCount(exposureKey.IntervalCount),
			CreatedAt:                 createdAt,
			LocalProvenance:           true, // This is the origin system for this data.
			VerificationAuthorityName: strings.ToUpper(strings.TrimSpace(inData.VerificationAuthorityName)),
		}
		entities = append(entities, infection)
	}
	return entities, nil
}
