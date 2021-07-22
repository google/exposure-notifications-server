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

// Package v1 contains API definitions that can be used outside of this codebase.
// The v1 API is considered stable.
// It will only add new optional fields and no fields will be removed.
package v1

import (
	"fmt"

	"github.com/golang-jwt/jwt"
)

const (
	// ExposureKeyHMACClaim is the JWT claim key for the HMAC of the TEKs
	ExposureKeyHMACClaim = "tekmac"
	// TransmissionRiskOverrideClaim is the JWT Claim key for transmission risk overrides
	TransmissionRiskOverrideClaim = "trisk"
	// ReportTypeClaim is the JWT claim for the report type (confirmed|likely|negative)
	ReportTypeClaim = "reportType"
	// SymptomOnsetIntervalClaim is the JWT claim for the interval representing the symptom onset.
	SymptomOnsetIntervalClaim = "symptomOnsetInterval"
	// TestDateIntervalClaim is the JWT claim for the interval representing the test date
	TestDateIntervalClaim = "testDateInterval"
	// KeyIDHeader is the standard JWT key ID header name.
	KeyIDHeader = "kid"

	// ReportType strings that correspond to what is defined in internal/pb/export/export.proto

	// ReportTypeConfirmed indicates to set ReportType.CONFIRMED_TEST
	ReportTypeConfirmed = "confirmed"
	// ReportTypeClinical indicates to set ReportType.CONFIRMED_CLINICAL_DIAGNOSIS
	ReportTypeClinical = "likely"
	// ReportTypeNegative is allowed by the verification flow. These keys are not saved in the system.
	ReportTypeNegative = "negative"
	// ReportTypeSelfReport indicates to set ReportType.SELF_REPORT
	ReportTypeSelfReport = "user-report"

	TransmissionRiskUnknown           = 0
	TransmissionRiskConfirmedStandard = 2
	TransmissionRiskClinical          = 4
	TransmissionRiskSelfReport        = 5
	TransmissionRiskNegative          = 6
)

var ValidReportTypes = map[string]bool{
	ReportTypeConfirmed:  true,
	ReportTypeClinical:   true,
	ReportTypeNegative:   true,
	ReportTypeSelfReport: true,
}

// VerificationClaims represents the accepted Claims portion of the verification certificate JWT.
// This data is used to set data on the uploaded TEKs and will be reflected on export. See the export file format:
// https://github.com/google/exposure-notifications-server/blob/main/internal/pb/export/export.proto#L73
type VerificationClaims struct {
	jwt.StandardClaims

	// ReportType is one of 'confirmed', 'likely', or 'negative' as defined by the constants in this file.
	// Required. Claims must contain a valid report type or the publish request won't have any effect.
	ReportType string `json:"reportType"`
	// SymptomOnsetInterval uses the same 10 minute interval timing as TEKs use. If an interval is provided that isn not the
	// start of a UTC day, then it will be rounded down to the beginning of that UTC day. And from there the days +/- symptom
	// onset will be calculated.
	// Optional. If present, TEKs will be adjusted accordingly on publish.
	SymptomOnsetInterval uint32 `json:"symptomOnsetInterval,omitempty"`

	// SignedMac is the HMAC of the TEKs that may be uploaded with the certificate containing these claims.
	// Required, indicates what can be uploaded with this certificate.
	SignedMAC string `json:"tekmac"`
}

// NewVerificationClaims initializes a new VerificationClaims struct.
func NewVerificationClaims() *VerificationClaims {
	return &VerificationClaims{}
}

// CustomClaimsValid returns nil if the custom claims are valid.
// .Valid() should still be called to validate the standard claims.
func (v *VerificationClaims) CustomClaimsValid() error {
	if !ValidReportTypes[v.ReportType] {
		return fmt.Errorf("invalid report type: %q", v.ReportType)
	}
	return nil
}
