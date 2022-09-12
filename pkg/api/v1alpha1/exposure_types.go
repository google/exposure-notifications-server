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

package v1alpha1

import "time"

// The following constants are generally useful in implementations of this API
// and for clients as well..
const (
	// only valid exposure key keyLength.
	KeyLength = 16

	// Transmission risk constraints (inclusive..inclusive).
	MinTransmissionRisk = 0 // 0 indicates, no/unknown risk.
	MaxTransmissionRisk = 8

	// Intervals are defined as 10 minute periods, there are 144 of them in a day.
	// IntervalCount constraints (inclusive..inclusive).
	MinIntervalCount = 1
	MaxIntervalCount = 144

	// Self explanatory.
	// oneDay = time.Hour * 24.

	// interval length.
	IntervalLength = 10 * time.Minute
)

// Publish represents the body of the PublishInfectedIds API call. Please see
// the individual fields below for details on their values.
type Publish struct {
	// Keys (temporaryExposureKeys) is the list of TEKs and is required. The array
	// must have more than 1 element and less than 21 elements
	// (maxKeysPerPublish).
	Keys []ExposureKey `json:"temporaryExposureKeys"`

	// Regions (regions) is the list of regions for the upload. This must match
	// the system configuration.
	Regions []string `json:"regions"`

	// AppPackageName (appPackageName) is the identifier for the mobile
	// application:
	//
	//   - Android: The App Package AppPackageName
	//   - iOS: The BundleID
	//
	AppPackageName string `json:"appPackageName"`

	// VerificationPayload (verificationPayload) is the certificate from a
	// verification server.
	VerificationPayload string `json:"verificationPayload"`

	// HMACKey (hmacKey) is the device-generated secret that is used to
	// recalculate the HMAC value that is present in the verification payload.
	HMACKey string `json:"hmackey"`

	// SymptomOnsetInterval (symptomOnsetInterval) is an interval number that
	// aligns with the symptom onset date:
	//
	//   - Uses the same interval system as TEK timing.
	//   - Will be rounded down to the start of the UTC day provided.
	//   - Will be used to calculate the days +/- symptom onset for provided keys.
	//   - MUST be no more than 14 days ago.
	//   - Does not have to be within range of any of the provided keys (i.e.
	//     future key uploads)
	//
	SymptomOnsetInterval int32 `json:"symptomOnsetInterval"`

	// RevisionToken (revisionToken) is an opaque string that must be passed
	// intact on additional publish requests from the same device, where the same
	// TEKs may be published again.
	RevisionToken string `json:"revisionToken"`

	// Padding (padding) is random, base64-encoded data to obscure the request
	// size. The server will not process this data in any way. The recommendation
	// is that padding be at least 1kb in size with a random jitter of at least
	// 1kb. Maximum overall request size is capped at 64kb for the serialized
	// JSON.
	Padding string `json:"padding"`

	// Platform (platform) must be one of "ios" or "android".
	//
	// DEPRECATED: This field has been deprecated.
	Platform string `json:"platform"`

	// DeviceVerificationPayload is the DeviceCheck or SafetyNet attestion.
	//
	// DEPRECATED: This field has been deprecated.
	DeviceVerificationPayload string `json:"deviceVerificationPayload"`
}

// PublishResponse is sent back to the client on a publish request. If
// successful, the revisionToken indicates an opaque string that must be passed
// back if the same devices wishes to publish TEKs again.
//
// On error, the error field will contain the error details.
//
// The Padding field may be populated with random data on both success and error
// responses.
//
// The Warnings field may be populated with a list of warnings. These are not
// errors, but may indicate the server mutated the response.
type PublishResponse struct {
	RevisionToken     string   `json:"revisionToken"`
	InsertedExposures int      `json:"insertedExposures"`
	Error             string   `json:"error"`
	Padding           string   `json:"padding"`
	Warnings          []string `json:"warnings,omitempty"`
}

// ExposureKey is the 16 byte key, the start time of the key and the duration of
// the key. A duration of 0 means 24 hours.
type ExposureKey struct {
	// Key (key) is the base64-encoded 16 byte exposure key from the device. The
	// base64 encoding should include padding, as per RFC 4648. If the key is not
	// exactly 16 bytes in length, the whole batch will fail.
	Key string `json:"key"`

	// IntervalNumber (rollingStartNumber) must be "reasonable" as in the system
	// won't accept keys that are scheduled to start in the future or that are too
	// far in the past, which is configurable per installation.
	IntervalNumber int32 `json:"rollingStartNumber"`

	// IntervalCount (rollingPeriod) must >= minIntervalCount and <=
	// maxIntervalCount, 1 - 144 inclusive.
	IntervalCount int32 `json:"rollingPeriod"`

	// TransmissionRisk (transmissionRisk) must be >= 0 and <= 8. This field is
	// optional, but should still be populated for compatibility with older
	// clients. If it is omitted, and there is a valid report type, then
	// transmissionRisk will be set to 0. If there is a report type from the
	// verification certificate AND tranismission risk is not set, then a report
	// type of:
	//
	//   - CONFIRMED will lead to transmission risk 2
	//   - LIKELY will lead to transmission risk 4
	//   - NEGATIVE will lead to transmission risk 6
	//
	TransmissionRisk int `json:"transmissionRisk,omitempty"` // DEPRECATED
}

// ExposureKeys represents a set of ExposureKey objects as input to
// export file generation utility.
// Keys: Required and must have length >= 1.
type ExposureKeys struct {
	Keys []ExposureKey `json:"temporaryExposureKeys"`
}
