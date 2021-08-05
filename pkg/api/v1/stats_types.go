// Copyright 2021 the Exposure Notifications Server authors
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

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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

// StatsDays represents a logical collection of stats.
type StatsDays []*StatsDay

// StatsResponse returns all currently known metrics for the authenticated health authority.
//
// There may be gaps in the Days if a day has insufficient data.
type StatsResponse struct {
	// Individual days. There may be gaps if a day does not have enough data.
	Days StatsDays `json:"days,omitempty"`

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

func (s *StatsDay) IsEmpty() bool {
	if s == nil {
		return true
	}

	if s.PublishRequests.Total() > 0 {
		return false
	}

	return true
}

// PublishRequests is a summary of one day's publish requests by platform.
type PublishRequests struct {
	UnknownPlatform int64 `json:"unknown"`
	Android         int64 `json:"android"`
	IOS             int64 `json:"ios"`
}

// Total returns the number of publish requests across all platforms.
func (p *PublishRequests) Total() int64 {
	return p.UnknownPlatform + p.Android + p.IOS
}

// TEKAgeDistributionAsString returns an array of TEKAgeDistribution
// as strings instead of int64
func (s *StatsDay) TEKAgeDistributionAsString() []string {
	rtn := make([]string, 0, len(s.TEKAgeDistribution))
	for _, v := range s.TEKAgeDistribution {
		rtn = append(rtn, strconv.FormatInt(v, 10))
	}
	return rtn
}

// OnsetToUploadDistributionAsString returns an array of OnsetToUploadDistribution
// as strings instead of int64
func (s *StatsDay) OnsetToUploadDistributionAsString() []string {
	rtn := make([]string, 0, len(s.OnsetToUploadDistribution))
	for _, v := range s.OnsetToUploadDistribution {
		rtn = append(rtn, strconv.FormatInt(v, 10))
	}
	return rtn
}

// MarshalCSV returns bytes in CSV format.
func (s StatsDays) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{
		"day",
		"publish_requests_unknown", "publish_requests_android", "publish_requests_ios",
		"total_teks_published", "requests_with_revisions", "requests_missing_onset_date", "tek_age_distribution", "onset_to_upload_distribution",
	}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Day.Format("2006-01-02"),
			strconv.FormatInt(stat.PublishRequests.UnknownPlatform, 10),
			strconv.FormatInt(stat.PublishRequests.Android, 10),
			strconv.FormatInt(stat.PublishRequests.IOS, 10),
			strconv.FormatInt(stat.TotalTEKsPublished, 10),
			strconv.FormatInt(stat.RevisionRequests, 10),
			strconv.FormatInt(stat.RequestsMissingOnsetDate, 10),
			strings.Join(stat.TEKAgeDistributionAsString(), "|"),
			strings.Join(stat.OnsetToUploadDistributionAsString(), "|"),
		}); err != nil {
			return nil, fmt.Errorf("failed to write CSV entry %d: %w", i, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("failed to create CSV: %w", err)
	}

	return b.Bytes(), nil
}
