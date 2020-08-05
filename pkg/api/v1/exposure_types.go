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

package v1

import "time"

// The following constants are generally useful in implementations of this API
// and for clients as well..
const (
	// only valid exposure key keyLength
	KeyLength = 16

	// Transmission risk constraints (inclusive..inclusive)
	MinTransmissionRisk = 0 // 0 indicates, no/unknown risk.
	MaxTransmissionRisk = 8

	// Intervals are defined as 10 minute periods, there are 144 of them in a day.
	// IntervalCount constraints (inclusive..inclusive)
	MinIntervalCount = 1
	MaxIntervalCount = 144

	// interval length
	IntervalLength = 10 * time.Minute

	// Error Code defintiions.
	// ErrorUnknownHealthAuthorityID indicates that the health authority was not found.
	ErrorUnknownHealthAuthorityID = "unknown_health_authority_id"
	// ErrorUnableToLoadHealthAuthority indicates a retryable error loading the configuration.
	ErrorUnableToLoadHealthAuthority = "unable_to_load_health_authority"
	// ErrorHealthAuthorityMissingRegionConfiguration indicautes the request can not accepted because
	// the specified health authority is not configured correctly.
	ErrorHealthAuthorityMissingRegionConfiguration = "health_authority_missing_region_config"
	// ErrorVerificationCertificateInvalid indicates a problem with the verification certificate.
	ErrorVerificationCertificateInvalid = "health_authority_verification_certificate_invalid"
	// ErrorBadRequest indicates that the client sent a request that couldn't be parsed correctly
	// or otherwise contains invalid data, see the extended ErrorMessage for details.
	ErrorBadRequest = "bad_request"
	// ErrorInternalError
	ErrorInternalError = "internal_error"
)

// Publish represents the body of the PublishInfectedIds API call.
// temporaryExposureKeys: Required and must have length >= 1 and <= 21 (`maxKeysPerPublish`)
// healthAuthorityID: The identifier for the mobile application.
//  - Android: The App Package AppPackageName
//  - iOS: The BundleID
// verificationPayload: The Verification Certificate from a verification server.
// hmacKey: the device generated secret that is used to recalcualte the HMAC value
//  that is present in the verification payload.
//
// symptomOnsetInterval: An interval number that aligns with the symptom onset date.
//  - Uses the same interval system as TEK timing.
//  - Will be rounded down to the start of the UTC day provided.
//  - Will be used to calculate the days +/- symptom onset for provided keys.
//  - MUST be no more than 14 days ago.
//  - Does not have to be within range of any of the provided keys (i.e. future
//    key uploads)
//
// traveler - set to true if the TEKs in this publish set are consider to be the
//  keys of a "traveler" who has left the home region represented by this server
//  (or by the home health authority in case of a multi-tenant installation).
//
// revisionToken: An opaque string that must be passed in-tact from on additional
//   publish requests from the same device, there the same TEKs may be published
//   again.
//
// Padding: random base64 encoded data to obscure the request size. The server will
// not process this data in any way.
type Publish struct {
	Keys                 []ExposureKey `json:"temporaryExposureKeys"`
	HealthAuthorityID    string        `json:"healthAuthorityID"`
	VerificationPayload  string        `json:"verificationPayload,omitempty"`
	HMACKey              string        `json:"hmacKey,omitempty"`
	SymptomOnsetInterval int32         `json:"symptomOnsetInterval,omitempty"`
	Traveler             bool          `json:"traveler,omitempty"`
	RevisionToken        string        `json:"revisionToken"`

	Padding string `json:"padding"`
}

// PublishResponse is sent back to the client on a publish request.
// If successful, the revisionToken indicates an opaque string that must be
// passed back if the same devices wishes to publish TEKs again.
//
// On error, the error message will contain a message from the server
// and the 'code' field will contain one of the constants defined in this file.
// The intent is that code can be used to show a localized error message on the
// device.
//
// The Padding field may be populated with random data on both success and
// error responses.
type PublishResponse struct {
	RevisionToken     string `json:"revisionToken,omitempty"`
	InsertedExposures int    `json:"insertedExposures,omitempty"`
	ErrorMessage      string `json:"error,omitempty"`
	Code              string `json:"code,omitempty"`
	Padding           string `json:"padding,omitempty"`
}

// ExposureKey is the 16 byte key, the start time of the key and the
// duration of the key. A duration of 0 means 24 hours.
// - ALL fields are REQUIRED and must meet the constraints below.
// Key must be the base64 (RFC 4648) encoded 16 byte exposure key from the device.
// - Base64 encoding should include padding, as per RFC 4648
// - if the key is not exactly 16 bytes in length, the request will be failed
// - that is, the whole batch will fail.
// IntervalNumber must be "reasonable" as in the system won't accept keys that
//   are scheduled to start in the future or that are too far in the past, which
//   is configurable per installation.
// IntervalCount must >= `minIntervalCount` and <= `maxIntervalCount`
//   1 - 144 inclusive.
// transmissionRisk must be >= 0 and <= 8.
//   Transmission risk is optional, but should still be populated for compatibility
//   with older clients. If it is omitted, and there is a valid report type,
//   then transmissionRisk will be set to 0.
//   IF there is a report type from the verification certificate AND tranismission risk
//    is not set, then a report type of
//     CONFIRMED will lead to TR 2
//     LIKELY will lead to TR 4
//     NEGATIVE will lead to TR 6
type ExposureKey struct {
	Key              string `json:"key"`
	IntervalNumber   int32  `json:"rollingStartNumber"`
	IntervalCount    int32  `json:"rollingPeriod"`
	TransmissionRisk int    `json:"transmissionRisk,omitempty"` // Optional
}

// ExposureKeys represents a set of ExposureKey objects as input to
// export file generation utility.
// Keys: Required and must have length >= 1.
type ExposureKeys struct {
	Keys []ExposureKey `json:"temporaryExposureKeys"`
}
