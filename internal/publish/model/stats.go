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

// Package model is a model abstraction for health authority telemetry.
package model

import (
	"sort"
	"time"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
)

const (
	// StatsMaxOldestTEK represents the oldest age (days) that will be reflected in stats.
	// Anything >= will count in the largest bucket.
	StatsMaxOldestTEK = 15
	// StatsMaxOnsetDays represents the oldest symptom onset age that will be reflected in stats.
	// Anything >= will count in the largest bucket.
	StatsMaxOnsetDays = 29

	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformUnknown = "unknown"
)

// Turns a platform identifier string into an int for calculation.
func platformToInt(platform string) int {
	switch platform {
	case PlatformAndroid:
		return 1
	case PlatformIOS:
		return 2
	default:
		return 0
	}
}

// HealthAuthorityStats represents the raw metrics for an individual
// health authority for a given hour.
type HealthAuthorityStats struct {
	HealthAuthorityID int64
	Hour              time.Time
	PublishCount      []int64
	TEKCount          int64
	RevisionCount     int64
	OldestTekDays     []int64
	OnsetAgeDays      []int64
	MissingOnset      int64
}

// ReduceStats takes hourly breakdowns and rolls them up to daily. The onlyBefore
// time indicates the cutoff point for inclusion.
// The dayThreshold indicates how many entries are needed to include a given day.
// Day boundaries are all in UTC.
func ReduceStats(hourly []*HealthAuthorityStats, onlyBefore time.Time, dayThreshold int64, embargoPeriod time.Duration) []*verifyapi.StatsDay {
	embargoRelease := embargoPeriod > 0
	now := time.Now().UTC()

	days := make(map[time.Time]*verifyapi.StatsDay)
	// Combine the hours into days based on same UTC midnight time.
	for _, hour := range hourly {
		if !hour.Hour.Before(onlyBefore) {
			continue
		}

		day := timeutils.UTCMidnight(hour.Hour)
		if _, ok := days[day]; !ok {
			// Initialize this day
			days[day] = &verifyapi.StatsDay{
				Day:                       day,
				TEKAgeDistribution:        make([]int64, StatsMaxOldestTEK+1),
				OnsetToUploadDistribution: make([]int64, StatsMaxOnsetDays+1),
			}
		}

		metricsDay := days[day]
		metricsDay.PublishRequests.Android += hour.PublishCount[platformToInt(PlatformAndroid)]
		metricsDay.PublishRequests.IOS += hour.PublishCount[platformToInt(PlatformIOS)]
		metricsDay.PublishRequests.UnknownPlatform += hour.PublishCount[platformToInt(PlatformUnknown)]
		metricsDay.TotalTEKsPublished += hour.TEKCount
		metricsDay.RevisionRequests += hour.RevisionCount
		metricsDay.RequestsMissingOnsetDate += hour.MissingOnset

		for i := 0; i <= StatsMaxOldestTEK && i < len(hour.OldestTekDays); i++ {
			metricsDay.TEKAgeDistribution[i] += hour.OldestTekDays[i]
		}
		for i := 0; i <= StatsMaxOnsetDays && i < len(hour.OnsetAgeDays); i++ {
			metricsDay.OnsetToUploadDistribution[i] += hour.OnsetAgeDays[i]
		}
	}

	// Bring the map back to an array
	result := make([]*verifyapi.StatsDay, 0, len(days))
	for _, day := range days {
		endOfDay := timeutils.UTCMidnight(day.Day.Add(24 * time.Hour))
		embargoOver := endOfDay.Add(embargoPeriod).Before(now)
		okToShow := day.PublishRequests.Total() >= dayThreshold || (embargoRelease && embargoOver)
		if !okToShow {
			continue
		}
		result = append(result, day)
	}
	// Sort before returning.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Day.Before(result[j].Day)
	})

	return result
}

// InitHour creates a HealthAuthorityStats record for specified hour.
func InitHour(healthAuthorityID int64, hour time.Time) *HealthAuthorityStats {
	return &HealthAuthorityStats{
		HealthAuthorityID: healthAuthorityID,
		Hour:              hour.UTC().Truncate(time.Hour),
		PublishCount:      make([]int64, 3),
		TEKCount:          0,
		RevisionCount:     0,
		OldestTekDays:     make([]int64, StatsMaxOldestTEK+1),
		OnsetAgeDays:      make([]int64, StatsMaxOnsetDays+1),
		MissingOnset:      0,
	}
}

// PublishInfo is the paremeters to the AddPublish call.
type PublishInfo struct {
	CreatedAt    time.Time
	Platform     string
	NumTEKs      int32
	Revision     bool
	OldestDays   int
	OnsetDaysAgo int
	MissingOnset bool
}

// AddPublish increments the stats for a given hour. This should be called
// inside of a read-modify-write database transaction. The HealthAuthorityStats
// represents the current state in the database, and the PublishInfo provided is
// added to it.
//
// The HealthAuthorityStats must be created by InitHour or may not be initialized correctly.
//
// This method does not enforce that it is called in a transaction, it only
// applyes the in-memory logic.
func (has *HealthAuthorityStats) AddPublish(info *PublishInfo) {
	has.PublishCount[platformToInt(info.Platform)]++

	has.TEKCount += int64(info.NumTEKs)
	if info.Revision {
		has.RevisionCount++
		return
	}
	// This info is only updated if it's not a revision.
	if age, length := info.OldestDays, len(has.OldestTekDays); age >= 0 && age < length {
		has.OldestTekDays[info.OldestDays]++
	} else if age >= length {
		// count this in the last, >= bucket.
		has.OldestTekDays[length-1]++
	}
	if info.MissingOnset {
		has.MissingOnset++
	} else {
		if oAge, length := info.OnsetDaysAgo, len(has.OnsetAgeDays); oAge >= 0 && oAge < length {
			has.OnsetAgeDays[oAge]++
		} else if oAge >= length {
			has.OnsetAgeDays[length-1]++
		}
	}
}
