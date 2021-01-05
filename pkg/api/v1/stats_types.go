// Copyright 2021 Google LLC
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

const (
	// ErrorUnauthorized is returned if the provided bearer token is invalid.
	ErrorUnauthorized = "unauthorized"
)

// StatsRequest represents the request to retrieve publish metrics for a specific
// health authority.
//
// Calls to this API require an "Authorization: Bearer <JWT>" header
// with a JWT signed with the same private used to sign verification certificates
// for the health authority.
//
// New stats are released every hour. And stats for a day (UTC) only start to be released
// once there have been a sufficient numbers of publish requests for that day.
//
// This API is invoked via POST request to /v1/stats
type StatsRequest struct {
	// currently no data fields in the request.

	Padding string `json:"padding"`
}

// StatsResponse returns all currently known metrics for the authenticated health authority.
//
// There may be gaps in the Days if a day has insufficient data.
type StatsResponse struct {
	// Individual days. There may be gaps if a day does not have enough data.
	Days []*StatsDay `json:"days,omitempty"`

	ErrorMessage string `json:"error,omitempty"`
	ErrorCode    string `json:"code,omitempty"`

	Padding string `json:"padding"`
}

// StatsDay represents stats from an individual day. All stats represent only successful requests.
type StatsDay struct {
	// Day will be set to midnight UTC of the day represented. An individual day
	// isn't released until there is a minimum threshold for updates has been met.
	Day                time.Time       `json:"day"`
	PublishRequests    PublishRequests `json:"publish_requests"`
	TotalTEKsPublished int64           `json:"total_teks_published"`
	// RevisionRequests is the number of publish requests that contained at least one TEK revision.
	RevisionRequests int64 `json:"requests_with_revisions"`
	// TEKAgeDistribution shows a distribution of the oldest tek in an upload.
	// The count at index 0-15 represent the number of uploads there the oldest TEK is that value.
	// Index 16 represents > 15 days.
	TEKAgeDistribution []int64 `json:"tek_age_distribution"`
	// OnsetToUploadDistribution shows a distribution of onset to upload, the index is in days.
	// The count at index 0-29 represents the number of uploads with that symptom onset age.
	// Index 30 represents > 29 days.
	OnsetToUploadDistribution []int64 `json:"onset_to_upload_distribution"`

	// RequestsMissingOnsetDate is the number of publish requests where no onset date
	// was provided. These request are not included in the onset to upload distribution.
	RequestsMissingOnsetDate int64 `json:"requests_missing_onset_date"`
}

type PublishRequests struct {
	UnknownPlatform int64 `json:"unknown"`
	Android         int64 `json:"android"`
	IOS             int64 `json:"ios"`
}

func (p *PublishRequests) Total() int64 {
	return p.UnknownPlatform + p.Android + p.IOS
}
