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

package v1alpha1

import "time"

// The following constants are generally useful in implementations of this API
// and for clients as well..
const (
	// 15 Days worth of keys is the maximum per publish request (inclusive)
	MaxKeysPerPublish = 15

	// only valid exposure key keyLength
	KeyLength = 16

	// Transmission risk constraints (inclusive..inclusive)
	MinTransmissionRisk = 0 // 0 indicates, no/unknown risk.
	MaxTransmissionRisk = 8

	// Intervals are defined as 10 minute periods, there are 144 of them in a day.
	// IntervalCount constraints (inclusive..inclusive)
	MinIntervalCount = 1
	MaxIntervalCount = 144

	// Self explanatory.
	// oneDay = time.Hour * 24

	// interval length
	IntervalLength = 10 * time.Minute
)

// Publish represents the body of the PublishInfectedIds API call.
// Keys: Required and must have length >= 1 and <= 21 (`maxKeysPerPublish`)
// Regions: Array of regions. System defined, must match configuration.
// AppPackageName: The identifier for the mobile application.
//  - Android: The App Package AppPackageName
//  - iOS: The BundleID
// TransmissionRisk: An integer from 0-8 (inclusive) that represents
//  the transmission risk for this publish.
// Verification: The attestation payload for this request. (iOS or Android specific)
//   Base64 encoded.
// VerificationAuthorityName: a string that should be verified against the code provider.
//  Note: This project doesn't directly include a diagnosis code verification System
//        but does provide the ability to configure one in `serverevn.ServerEnv`
//
// The following fields are deprecated, but accepted for backwards-compatability:
// DeviceVerificationPayload: (attestation)
// Platform: "ios" or "android"
type Publish struct {
	Keys                []ExposureKey `json:"temporaryExposureKeys"`
	Regions             []string      `json:"regions"`
	AppPackageName      string        `json:"appPackageName"`
	VerificationPayload string        `json:"verificationPayload"`
	HMACKey             string        `json:"hmackey"`
	Padding             string        `json:"padding"`

	Platform                  string `json:"platform"`                  // DEPRECATED
	DeviceVerificationPayload string `json:"deviceVerificationPayload"` // DEPRECATED
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
type ExposureKey struct {
	Key              string `json:"key"`
	IntervalNumber   int32  `json:"rollingStartNumber"`
	IntervalCount    int32  `json:"rollingPeriod"`
	TransmissionRisk int    `json:"transmissionRisk"`
}

// ExposureKeys represents a set of ExposureKey objects as input to
// export file generation utility.
// Keys: Required and must have length >= 1.
type ExposureKeys struct {
	Keys []ExposureKey `json:"temporaryExposureKeys"`
}
