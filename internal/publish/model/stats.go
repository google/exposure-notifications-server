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

// Package model is a model abstraction for health authority telemetry.
package model

import (
	"time"
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

var (
	platforms = map[string]int{
		PlatformAndroid: 0,
		PlatformIOS:     1,
		PlatformUnknown: 2,
	}
)

// HealthAuthorityStats represents the raw metrics for an individual
// health authority for a given hour.
type HealthAuthorityStats struct {
	HealthAuthorityID int64
	Hour              time.Time
	PublishCount      []int32
	TEKCount          int32
	RevisionCount     int32
	OldestTekDays     []int32
	OnsetAgeDays      []int32
	MissingOnset      int32
}

// InitHour creates a HealthAuthorityStats record for specified hour.
func InitHour(healthAuthorityID int64, hour time.Time) *HealthAuthorityStats {
	return &HealthAuthorityStats{
		HealthAuthorityID: healthAuthorityID,
		Hour:              hour.UTC().Truncate(time.Hour),
		PublishCount:      make([]int32, len(platforms)),
		TEKCount:          0,
		RevisionCount:     0,
		OldestTekDays:     make([]int32, StatsMaxOldestTEK+1),
		OnsetAgeDays:      make([]int32, StatsMaxOnsetDays+1),
		MissingOnset:      0,
	}
}

// PublishInfo is the paremeters to the AddPublish call
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
// inside of a read-modify-write database transaction.
func (has *HealthAuthorityStats) AddPublish(info *PublishInfo) {
	switch info.Platform {
	case PlatformAndroid:
		has.PublishCount[platforms[PlatformAndroid]]++
	case PlatformIOS:
		has.PublishCount[platforms[PlatformIOS]]++
	default:
		has.PublishCount[platforms[PlatformUnknown]]++
	}

	has.TEKCount += info.NumTEKs
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
