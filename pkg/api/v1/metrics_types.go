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
	ErrorUnauthorized = "unauthorized"
)

// MetricsRequest represents the request to retrieve publish metrics for a specific
// health authority.
//
// Calls to this API require an "Authorization: Bearer <JWT>" header
// with a JWT signed with the same private used to sign verification certificates
// for the health authority.
//
// New stats are released every hour. And stats for a day (UTC) only start to be released
// once there have been a sufficient numbers of publish requests for that day.
type MetricsRequest struct {
	// currently no fields in the request.
}

// MetricsResponse returns all currently known metrics for the authenticated health authority.
//
// There may be gaps in the Days if a day has insufficient data.
type MetricsResponse struct {
	// Individual days. There may be gaps if a day does not have enough data.
	Days []MetricsDay `json:"days,omitempty"`

	ErrorMessage string `json:"error,omitempty"`
	ErrorCode    string `json:"code,omitempty"`
}

// MetricsDay represents stats from an individual day.
type MetricsDay struct {
	// Day will be set to midnight UTC of the day represented. An individual day
	// isn't released until there is a minimum threshold for updates has been met.
	Day          time.Time    `json:"day"`
	PublishCount PublishCount `json:"publishCount"`
	TEKs         int64        `json:"teks"`
	Revisions    int64        `json:"revisions"`
	// OldestTEKDistribution shows a distribution of the oldest tek in an upload.
	// The count at index 0-15 represent the number of uploads there the oldest TEK is that value.
	// Index 16 represents > 15.
	OldestTEKDistribution []int64 `json:"oldestTEKDistribution"`
	// OnsetToUpload shows a distribution of onset to upload, the index is in days.
	// The count at index 0-29 represents the number of uploads with that symptom onset age.
	// Index 30 represents > 29 days.
	OnsetToUpload []int64 `json:"onsetToUpload"`

	MissingOnset int64 `json:"missingOnset"`
}

type PublishCount struct {
	UnknownPlatform int64 `json:"unknown"`
	Android         int64 `json:"android"`
	IOS             int64 `json:"ios"`
}

func (p *PublishCount) Total() int64 {
	return p.UnknownPlatform + p.Android + p.IOS
}
