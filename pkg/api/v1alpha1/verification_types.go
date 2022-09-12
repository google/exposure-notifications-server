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

// Package v1alpha1 contains API definitions that can be used outside of this
// codebase in their alpha form.
// These APIs are not considered to be stable at HEAD.
package v1alpha1

import (
	"sort"

	"github.com/golang-jwt/jwt"
)

const (
	// ExposureKeyHMACClaim is the JWT claim key for the HMAC of the TEKs.
	ExposureKeyHMACClaim = "tekmac"
	// TransmissionRiskOverrideClaim is the JWT Claim key for transmission risk overrides.
	TransmissionRiskOverrideClaim = "trisk"
	// ReportTypeClaim is the JWT claim for the report type (confirmed|likely|negative).
	ReportTypeClaim = "reportType"
	// SymptomOnsetIntervalClaim is the JWT claim for the interval representing the symptom onset.
	SymptomOnsetIntervalClaim = "symptomOnsetInterval"
	// TestDateIntervalClaim is the JWT claim for the interval representing the test date.
	TestDateIntervalClaim = "testDateInterval"
	// KeyIDHeader is the standard JWT key ID header name.
	KeyIDHeader = "kid"

	// ReportType strings that correspond to what is defined in internal/pb/export/export.proto.

	// ReportTypeConfirmed indicates to set ReportType.CONFIRMED_TEST.
	ReportTypeConfirmed = "confirmed"
	// ReportTypeClinical indicates to set ReportType.CONFIRMED_CLINICAL_DIAGNOSIS.
	ReportTypeClinical = "likely"
	// ReportTypeNegative is allowed by the verification flow. These keys are not saved in the system.
	ReportTypeNegative = "negative"

	TransmissionRiskUnknown           = 0
	TransmissionRiskConfirmedStandard = 2
	TransmissionRiskClinical          = 4
	TransmissionRiskNegative          = 6
)

// TransmissionRiskVector is an additional set of claims that can be
// included in the verification certificate for a diagnosis as received
// from a trusted public health authority.
// DEPRECATED - If received at a server, these values are ignored. Will be removed in v0.3.
type TransmissionRiskVector []TransmissionRiskOverride

// Compile time check that TransmissionRiskVector implements the sort interface.
var _ sort.Interface = TransmissionRiskVector{}

// TransmissionRiskOverride is an individual transmission risk override.
type TransmissionRiskOverride struct {
	TransmissionRisk     int   `json:"tr"`
	SinceRollingInterval int32 `json:"sinceRollingInterval"`
}

func (a TransmissionRiskVector) Len() int {
	return len(a)
}

// Less sorts the TransmissionRiskVector vector with the largest SinceRollingPeriod
// value first. Descending sort.
func (a TransmissionRiskVector) Less(i, j int) bool {
	return a[i].SinceRollingInterval > a[j].SinceRollingInterval
}

func (a TransmissionRiskVector) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// VerificationClaims represents the accepted Claims portion of the verification
// certificate JWT. This data is used to set data on the uploaded TEKs and will
// be reflected on export. See the [export file format].
//
// [export file format]: https://github.com/google/exposure-notifications-server/blob/main/internal/pb/export/export.proto#L73
type VerificationClaims struct {
	// ReportType is one of 'confirmed', 'likely', or 'negative' as defined by the
	// constants in this file.
	ReportType string `json:"reportType"`

	// SymptomOnsetInterval uses the same 10 minute interval timing as TEKs use.
	// If an interval is provided that isn not the start of a UTC day, then it
	// will be rounded down to the beginning of that UTC day. And from there the
	// days +/- symptom onset will be calculated.
	SymptomOnsetInterval uint32 `json:"symptomOnsetInterval"`

	// Deprecated, but not scheduled for removal. TransmissionRisks will continue
	// to be supported. On newer versions of the device software, the ReportType
	// and days +/- symptom onset will be used.
	TransmissionRisks TransmissionRiskVector `json:"trisk,omitempty"`

	SignedMAC string `json:"tekmac"`

	jwt.StandardClaims
}

// NewVerificationClaims initializes a new VerificationClaims struct.
func NewVerificationClaims() *VerificationClaims {
	return &VerificationClaims{
		TransmissionRisks: []TransmissionRiskOverride{},
	}
}
